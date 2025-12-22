package common

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// CleanProfile 文本清洗配置文件
type CleanProfile int

const (
	ProfileCommon    CleanProfile = iota // MySQL + PostgreSQL 通用安全集
	ProfileJSON                          // JSON 专用（严格模式）
	ProfileEmbedding                     // 向量化友好（标准化空格和换行）
	ProfileDatabase                      // 数据库存储（包含PUA过滤）
)

var (
	// 多个空格/制表符合并为一个空格
	spaceRe = regexp.MustCompile(`[ \t\f\v]+`)
	// 多个换行符（3个或以上）合并为两个换行
	newlineRe = regexp.MustCompile(`\n{3,}`)
)

// 零宽字符集合
var zeroWidthRunes = map[rune]bool{
	'\u200B': true, // Zero Width Space
	'\u200C': true, // Zero Width Non-Joiner
	'\u200D': true, // Zero Width Joiner
	'\uFEFF': true, // Zero Width No-Break Space (BOM)
	'\u2060': true, // Word Joiner
	'\u180E': true, // Mongolian Vowel Separator
}

// 非标准空格字符映射（转换为普通空格）
var nonStandardSpaces = map[rune]bool{
	'\u00A0': true, // Non-breaking space
	'\u1680': true, // Ogham Space Mark
	'\u2000': true, // En Quad
	'\u2001': true, // Em Quad
	'\u2002': true, // En Space
	'\u2003': true, // Em Space
	'\u2004': true, // Three-Per-Em Space
	'\u2005': true, // Four-Per-Em Space
	'\u2006': true, // Six-Per-Em Space
	'\u2007': true, // Figure Space
	'\u2008': true, // Punctuation Space
	'\u2009': true, // Thin Space
	'\u200A': true, // Hair Space
	'\u202F': true, // Narrow No-Break Space
	'\u205F': true, // Medium Mathematical Space
	'\u3000': true, // Ideographic Space (全角空格)
}

// Emoji变体选择器范围
const (
	variationSelectorStart = '\uFE00'
	variationSelectorEnd   = '\uFE0F'
)

// CleanText 统一的文本清洗入口
// 参数：
//   - input: 待清洗的文本（byte数组）
//   - profile: 清洗配置文件
//
// 返回：
//   - 清洗后的文本
//   - 错误信息（如果有）
func CleanText(input []byte, profile CleanProfile) (string, error) {
	// 1. UTF-8 校验
	if !utf8.Valid(input) {
		return "", errors.New("invalid UTF-8 sequence")
	}

	s := string(input)

	// 2. NULL 字符清理（MySQL和PostgreSQL都不支持）
	s = strings.ReplaceAll(s, "\u0000", "")

	// 3. 控制字符清理（保留 \n, \t, \r）
	s = removeControlChars(s)

	// 4. 零宽字符 & BOM 清理
	s = removeZeroWidthChars(s)

	// 5. Unicode 归一化（NFC标准）
	s = norm.NFC.String(s)

	// 6. 根据Profile进行定制清洗
	switch profile {
	case ProfileJSON:
		if err := validateJSONSafe(s); err != nil {
			return "", err
		}

	case ProfileEmbedding:
		// 向量化友好：标准化空格和换行
		s = normalizeWhitespace(s)
		s = normalizeNonStandardSpaces(s)

	case ProfileDatabase:
		// 数据库存储：额外清理私有使用区字符和emoji变体
		s = removePrivateUseArea(s)
		s = removeEmojiVariationSelectors(s)
		s = normalizeNonStandardSpaces(s)

	case ProfileCommon:
		// 通用模式：基础清理 + 非标准空格转换
		s = normalizeNonStandardSpaces(s)
	}

	return s, nil
}

// CleanString 便捷方法：直接接受string参数
func CleanString(input string, profile CleanProfile) (string, error) {
	return CleanText([]byte(input), profile)
}

// ==========================
// 辅助函数
// ==========================

// removeControlChars 清理控制字符（保留 \n, \t, \r）
func removeControlChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		// 保留换行、制表符和回车
		if r == '\n' || r == '\t' || r == '\r' {
			b.WriteRune(r)
			continue
		}
		// 跳过其他控制字符（<0x20 或 0x7F）
		if r < 0x20 || r == 0x7F {
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

// removeZeroWidthChars 清理零宽字符
func removeZeroWidthChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if zeroWidthRunes[r] {
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

// normalizeNonStandardSpaces 将非标准空格转换为普通空格
func normalizeNonStandardSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if nonStandardSpaces[r] {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}

// normalizeWhitespace 标准化空格和换行符
func normalizeWhitespace(s string) string {
	// 统一换行符
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// 合并多个空格为一个
	s = spaceRe.ReplaceAllString(s, " ")

	// 合并多个换行为两个（保留段落分隔）
	s = newlineRe.ReplaceAllString(s, "\n\n")

	// 清理首尾空白
	return strings.TrimSpace(s)
}

// removePrivateUseArea 清理私有使用区字符 (PUA)
func removePrivateUseArea(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		// U+E000..U+F8FF (BMP私有区)
		// U+F0000..U+FFFFD (平面15私有区)
		// U+100000..U+10FFFD (平面16私有区)
		if (r >= 0xE000 && r <= 0xF8FF) ||
			(r >= 0xF0000 && r <= 0xFFFFD) ||
			(r >= 0x100000 && r <= 0x10FFFD) {
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

// removeEmojiVariationSelectors 清理Emoji变体选择器
func removeEmojiVariationSelectors(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if r >= variationSelectorStart && r <= variationSelectorEnd {
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

// validateJSONSafe 验证文本是否对JSON安全
func validateJSONSafe(s string) error {
	for _, r := range s {
		// JSON不允许未转义的控制字符（除了\n, \t, \r）
		if r < 0x20 && r != '\n' && r != '\t' && r != '\r' {
			return errors.New("invalid control character for JSON")
		}

		// 私有使用区字符在JSON中不推荐使用
		// U+E000..U+F8FF (BMP私有区)
		// U+F0000..U+FFFFD (平面15私有区)
		// U+100000..U+10FFFD (平面16私有区)
		if (r >= 0xE000 && r <= 0xF8FF) ||
			(r >= 0xF0000 && r <= 0xFFFFD) ||
			(r >= 0x100000 && r <= 0x10FFFD) {
			return errors.New("private use unicode not allowed in JSON")
		}
	}
	return nil
}

// MustCleanText CleanText的panic版本（用于确定输入有效的场景）
func MustCleanText(input []byte, profile CleanProfile) string {
	result, err := CleanText(input, profile)
	if err != nil {
		panic(err)
	}
	return result
}

// MustCleanString CleanString的panic版本
func MustCleanString(input string, profile CleanProfile) string {
	result, err := CleanString(input, profile)
	if err != nil {
		panic(err)
	}
	return result
}
