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

// QwenFormatter 通义千问消息格式适配器
// 负责将消息转换为通义千问要求的格式，特别是多模态消息
type QwenFormatter struct{}

// NewQwenFormatter 创建Qwen格式适配器
func NewQwenFormatter() *QwenFormatter {
	return &QwenFormatter{}
}

// FormatMessages 转换消息格式为通义千问兼容格式
func (f *QwenFormatter) FormatMessages(messages []*schema.Message) ([]openai.ChatCompletionMessage, error) {
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
func (f *QwenFormatter) formatSingleMessage(msg *schema.Message) (openai.ChatCompletionMessage, error) {
	openaiMsg := openai.ChatCompletionMessage{
		Role: string(msg.Role),
	}

	// 如果是 Tool 角色的消息，必须设置 ToolCallID
	if msg.Role == schema.Tool {
		openaiMsg.ToolCallID = msg.ToolCallID
		openaiMsg.Content = msg.Content
		return openaiMsg, nil
	}

	// 如果是 Assistant 角色且有 ToolCalls，需要转换
	if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
		openaiMsg.Content = msg.Content
		openaiMsg.ToolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			openaiMsg.ToolCalls[i] = openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolType(tc.Type),
				Function: openai.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
		return openaiMsg, nil
	}

	// 检查是否有多模态内容
	if len(msg.UserInputMultiContent) > 0 {
		// 使用多模态内容数组格式（通义千问要求的格式）
		contentParts := f.convertUserInputMultiContent(msg.UserInputMultiContent)
		openaiMsg.MultiContent = contentParts
	} else {
		// 普通文本消息
		// 对于User角色，在多模态场景下也应该使用数组格式
		if msg.Role == schema.User && msg.Content != "" {
			// 将纯文本也转换为数组格式以保持一致性
			openaiMsg.MultiContent = []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: msg.Content,
				},
			}
		} else {
			// 系统消息和助手消息使用普通字符串
			openaiMsg.Content = msg.Content
		}
	}

	return openaiMsg, nil
}

// convertUserInputMultiContent 转换UserInputMultiContent
func (f *QwenFormatter) convertUserInputMultiContent(parts []schema.MessageInputPart) []openai.ChatMessagePart {
	var contentParts []openai.ChatMessagePart

	for _, part := range parts {
		switch part.Type {
		case schema.MessagePartTypeText:
			contentParts = append(contentParts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: part.Text,
			})

		case schema.MessagePartTypeImageURL:
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

		case schema.MessagePartTypeAudioURL:
			// 通义千问可能不支持音频，作为文本描述
			contentParts = append(contentParts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: "[音频文件]",
			})

		case schema.MessagePartTypeVideoURL:
			// 通义千问可能不支持视频，作为文本描述
			contentParts = append(contentParts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: "[视频文件]",
			})
		}
	}

	return contentParts
}

// buildImageURL 构建图片URL
func (f *QwenFormatter) buildImageURL(image *schema.MessageInputImage) string {
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
		// 如果是文件路径，需要读取文件并转换为base64
		if len(urlStr) > 0 && (urlStr[0] == '/' || urlStr[0] == '.') {
			return f.filePathToDataURI(urlStr, image.MIMEType)
		}
		// 假设是有效的HTTP URL或data URI
		return urlStr
	}

	return ""
}

// filePathToDataURI 将文件路径转换为data URI
func (f *QwenFormatter) filePathToDataURI(filePath, mimeType string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		g.Log().Warningf(context.Background(), "Failed to read image file %s: %v, skipping", filePath, err)
		return ""
	}

	if mimeType == "" {
		ext := filepath.Ext(filePath)
		mimeType = getMimeType(ext)
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
}

// getMimeType 获取MIME类型
func getMimeType(ext string) string {
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
