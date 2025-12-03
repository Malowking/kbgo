package formatter

import (
	"context"

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
			if part.Image != nil && part.Image.URL != nil {
				contentParts = append(contentParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL:    *part.Image.URL,
						Detail: openai.ImageURLDetailAuto,
					},
				})
			}
		}
	}

	return contentParts
}
