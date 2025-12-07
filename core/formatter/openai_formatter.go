package formatter

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/sashabaranov/go-openai"
)

// OpenAIFormatter OpenAI标准消息格式适配器
// 负责将消息转换为OpenAI标准格式
type OpenAIFormatter struct{}

// NewOpenAIFormatter 创建OpenAI格式适配器
func NewOpenAIFormatter() *OpenAIFormatter {
	return &OpenAIFormatter{}
}

// FormatMessages 转换消息格式为OpenAI标准格式
func (f *OpenAIFormatter) FormatMessages(messages []*schema.Message) ([]openai.ChatCompletionMessage, error) {
	result := make([]openai.ChatCompletionMessage, 0, len(messages))

	for _, msg := range messages {
		openaiMsg, err := f.formatSingleMessage(msg)
		if err != nil {
			g.Log().Errorf(context.Background(), "Failed to convert message: %v", err)
			continue
		}
		result = append(result, openaiMsg)
	}

	return result, nil
}

// formatSingleMessage 转换单条消息
func (f *OpenAIFormatter) formatSingleMessage(msg *schema.Message) (openai.ChatCompletionMessage, error) {
	openaiMsg := openai.ChatCompletionMessage{
		Role: string(msg.Role),
	}

	// 检查是否有多模态内容
	if len(msg.MultiContent) > 0 {
		contentParts := f.convertMultiContent(msg.MultiContent)
		openaiMsg.MultiContent = contentParts
	} else if len(msg.UserInputMultiContent) > 0 {
		// 也支持UserInputMultiContent
		contentParts := f.convertUserInputMultiContent(msg.UserInputMultiContent)
		openaiMsg.MultiContent = contentParts
	} else {
		// 普通文本消息
		openaiMsg.Content = msg.Content
	}

	return openaiMsg, nil
}

// convertMultiContent 转换MultiContent
func (f *OpenAIFormatter) convertMultiContent(parts []schema.ChatMessagePart) []openai.ChatMessagePart {
	var contentParts []openai.ChatMessagePart

	for _, part := range parts {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			contentParts = append(contentParts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: part.Text,
			})

		case schema.ChatMessagePartTypeImageURL:
			if part.ImageURL != nil {
				contentParts = append(contentParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL:    part.ImageURL.URL,
						Detail: openai.ImageURLDetail(part.ImageURL.Detail),
					},
				})
			}
		}
	}

	return contentParts
}

// convertUserInputMultiContent 转换UserInputMultiContent
func (f *OpenAIFormatter) convertUserInputMultiContent(parts []schema.MessageInputPart) []openai.ChatMessagePart {
	var contentParts []openai.ChatMessagePart

	for _, part := range parts {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			contentParts = append(contentParts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: part.Text,
			})

		case schema.ChatMessagePartTypeImageURL:
			if part.Image != nil {
				imageURL := f.buildImageURL(part.Image)
				if imageURL != "" {
					contentParts = append(contentParts, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL:    imageURL,
							Detail: openai.ImageURLDetailAuto,
						},
					})
				}
			}
		}
	}

	return contentParts
}

// buildImageURL 构建图片URL
// 优先使用Base64Data，其次使用URL
func (f *OpenAIFormatter) buildImageURL(image *schema.MessageInputImage) string {
	// 优先使用Base64Data
	if image.Base64Data != nil && *image.Base64Data != "" {
		mimeType := image.MIMEType
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		return fmt.Sprintf("data:%s;base64,%s", mimeType, *image.Base64Data)
	}

	// 使用URL
	if image.URL != nil && *image.URL != "" {
		urlStr := *image.URL
		// 如果是本地文件路径，需要读取文件并转换为base64
		if len(urlStr) > 0 && (urlStr[0] == '/' || urlStr[0] == '.') {
			return f.filePathToDataURI(urlStr, image.MIMEType)
		}
		// HTTP URL或已经是data URI
		return urlStr
	}

	return ""
}

// filePathToDataURI 将文件路径转换为data URI
func (f *OpenAIFormatter) filePathToDataURI(filePath, mimeType string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		g.Log().Warningf(context.Background(), "Failed to read image file %s: %v, skipping", filePath, err)
		return ""
	}

	if mimeType == "" {
		ext := filepath.Ext(filePath)
		mimeType = getOpenAIMimeType(ext)
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
}

// getOpenAIMimeType 获取MIME类型
func getOpenAIMimeType(ext string) string {
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".webp": "image/webp",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "image/jpeg"
}
