package common

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Malowking/kbgo/pkg/schema"
)

// imageURLPattern 匹配Markdown格式的图片
// 匹配格式: ![image-0](http://127.0.0.1:8002/images/xxx.jpeg)
var imageURLPattern = regexp.MustCompile(`!\[image-\d+\]\((http://[^)]+/images/([^)]+))\)`)

// httpImageURLPattern 匹配独立的HTTP图片URL（不在Markdown语法中）
// 匹配格式: http://127.0.0.1:8002/images/xxx.jpeg
var httpImageURLPattern = regexp.MustCompile(`http://[^/]+/images/([a-f0-9]+\.jpeg)`)

// ConvertImageURLsInDocument 转换文档中的图片URL为可访问的代理URL
// baseURL: 当前服务的基础URL，例如 "http://localhost:8000"
func ConvertImageURLsInDocument(doc *schema.Document, baseURL string) {
	if doc == nil {
		return
	}
	doc.Content = ConvertImageURLsInContent(doc.Content, baseURL)
}

// ConvertImageURLsInDocuments 批量转换文档中的图片URL
func ConvertImageURLsInDocuments(docs []*schema.Document, baseURL string) {
	for _, doc := range docs {
		ConvertImageURLsInDocument(doc, baseURL)
	}
}

// ConvertImageURLsInContent 转换内容中的图片URL
// 1. 将 ![image-0](http://127.0.0.1:8002/images/xxx.jpeg) 转换为 ![image-0](http://localhost:8000/api/v1/images/xxx.jpeg)
// 2. 将 http://127.0.0.1:8002/images/xxx.jpeg 转换为 ![image](http://localhost:8000/api/v1/images/xxx.jpeg)
func ConvertImageURLsInContent(content string, baseURL string) string {
	// 确保 baseURL 没有尾部斜杠
	baseURL = strings.TrimRight(baseURL, "/")

	// 第一步：处理Markdown格式的图片URL
	result := imageURLPattern.ReplaceAllStringFunc(content, func(match string) string {
		// 提取匹配的子组
		matches := imageURLPattern.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match // 如果匹配失败，返回原始内容
		}

		originalURL := matches[1] // 完整的原始URL
		imageName := matches[2]   // 图片文件名

		// 构建新的代理URL
		proxyURL := fmt.Sprintf("%s/api/v1/images/%s", baseURL, imageName)

		// 替换为新的Markdown格式
		// 保留原始的 ![image-N] 部分，只替换URL
		return strings.Replace(match, originalURL, proxyURL, 1)
	})

	// 第二步：处理完整HTTP URL格式的图片（http://127.0.0.1:8002/images/xxx.jpeg）
	// 将其转换为Markdown格式
	result = httpImageURLPattern.ReplaceAllStringFunc(result, func(match string) string {
		// 提取文件名
		matches := httpImageURLPattern.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		imageName := matches[1] // xxx.jpeg

		// 构建新的代理URL
		proxyURL := fmt.Sprintf("%s/api/v1/images/%s", baseURL, imageName)

		// 返回Markdown格式的图片
		return fmt.Sprintf("![image](%s)", proxyURL)
	})

	return result
}

// ExtractImageURLs 从内容中提取所有图片URL
func ExtractImageURLs(content string) []string {
	matches := imageURLPattern.FindAllStringSubmatch(content, -1)
	urls := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			urls = append(urls, match[1]) // 完整的原始URL
		}
	}
	return urls
}

// GetBaseURL 从请求上下文获取基础URL
// 优先使用 X-Forwarded-Host 和 X-Forwarded-Proto，否则使用 Host 和 Scheme
func GetBaseURL(host, scheme string, headers map[string]string) string {
	// 检查代理头
	if forwardedHost, ok := headers["X-Forwarded-Host"]; ok && forwardedHost != "" {
		host = forwardedHost
	}
	if forwardedProto, ok := headers["X-Forwarded-Proto"]; ok && forwardedProto != "" {
		scheme = forwardedProto
	}

	// 默认使用 http
	if scheme == "" {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}
