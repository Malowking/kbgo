package common

import (
	"regexp"
	"strings"
)

// SanitizeMilvusString 转义 Milvus 表达式中的特殊字符
// 防止通过特殊字符进行表达式注入
func SanitizeMilvusString(s string) string {
	// 转义反斜杠（必须先转义）
	s = strings.ReplaceAll(s, `\`, `\\`)
	// 转义双引号
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// ValidateUUID 验证 UUID 格式（支持有连字符和无连字符两种格式）
// 返回 true 表示格式合法
// 支持格式:
// 1. 标准格式（带连字符）: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
// 2. 无连字符格式: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx (32个十六进制字符)
func ValidateUUID(uuid string) bool {
	lowerUUID := strings.ToLower(uuid)

	// 支持标准的 UUID 格式（带连字符）
	patternWithHyphen := `^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`
	if matched, _ := regexp.MatchString(patternWithHyphen, lowerUUID); matched {
		return true
	}

	// 支持无连字符的 UUID 格式（32个十六进制字符）
	patternWithoutHyphen := `^[a-f0-9]{32}$`
	if matched, _ := regexp.MatchString(patternWithoutHyphen, lowerUUID); matched {
		return true
	}

	return false
}

// ValidateCollectionName 验证集合名称（只允许字母、数字、下划线）
// Milvus 集合名称规范: 1-255 字符，字母开头，只能包含字母、数字、下划线
func ValidateCollectionName(name string) bool {
	if len(name) == 0 || len(name) > 255 {
		return false
	}
	// 必须以字母开头，只能包含字母、数字、下划线
	pattern := `^[a-zA-Z][a-zA-Z0-9_]*$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}
