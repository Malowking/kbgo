package model

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/sashabaranov/go-openai"
)

// QwenAdapter 通义千问多模态适配器
type QwenAdapter struct {
	client *openai.Client
}

// NewQwenAdapter 创建通义千问适配器
func NewQwenAdapter(apiKey, baseURL string) *QwenAdapter {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	return &QwenAdapter{
		client: openai.NewClientWithConfig(config),
	}
}

// ChatCompletion 使用通义千问格式进行对话
func (a *QwenAdapter) ChatCompletion(ctx context.Context, modelName string, messages []*schema.Message, temperature float64, maxTokens int) (*openai.ChatCompletionResponse, error) {
	// 转换消息格式
	openaiMessages, err := a.convertMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// 构建请求
	req := openai.ChatCompletionRequest{
		Model:       modelName,
		Messages:    openaiMessages,
		Temperature: float32(temperature),
		MaxTokens:   maxTokens,
	}

	// 调用API
	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	return &resp, nil
}

// ChatCompletionStream 使用通义千问格式进行流式对话
func (a *QwenAdapter) ChatCompletionStream(ctx context.Context, modelName string, messages []*schema.Message, temperature float64, maxTokens int) (*openai.ChatCompletionStream, error) {
	// 转换消息格式
	openaiMessages, err := a.convertMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// 构建请求
	req := openai.ChatCompletionRequest{
		Model:       modelName,
		Messages:    openaiMessages,
		Temperature: float32(temperature),
		MaxTokens:   maxTokens,
		Stream:      true,
	}

	// 调用API
	stream, err := a.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion stream: %w", err)
	}

	return stream, nil
}

// convertMessages 转换消息格式为通义千问兼容格式
func (a *QwenAdapter) convertMessages(messages []*schema.Message) ([]openai.ChatCompletionMessage, error) {
	result := make([]openai.ChatCompletionMessage, 0, len(messages))

	for _, msg := range messages {
		openaiMsg, err := a.convertSingleMessage(msg)
		if err != nil {
			g.Log().Errorf(context.Background(), "Failed to convert message: %v", err)
			continue
		}
		result = append(result, openaiMsg)
	}

	return result, nil
}

// convertSingleMessage 转换单条消息
func (a *QwenAdapter) convertSingleMessage(msg *schema.Message) (openai.ChatCompletionMessage, error) {
	openaiMsg := openai.ChatCompletionMessage{
		Role: string(msg.Role),
	}

	// 检查是否有多模态内容
	if len(msg.UserInputMultiContent) > 0 {
		// 使用多模态内容数组格式（通义千问要求的格式）
		var contentParts []openai.ChatMessagePart

		for _, part := range msg.UserInputMultiContent {
			switch part.Type {
			case schema.ChatMessagePartTypeText:
				contentParts = append(contentParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: part.Text,
				})

			case schema.ChatMessagePartTypeImageURL:
				if part.Image != nil {
					var imageURL string

					// 优先使用Base64Data
					if part.Image.Base64Data != nil && *part.Image.Base64Data != "" {
						// 构造data URI格式
						mimeType := part.Image.MIMEType
						if mimeType == "" {
							mimeType = "image/jpeg"
						}
						imageURL = fmt.Sprintf("data:%s;base64,%s", mimeType, *part.Image.Base64Data)
					} else if part.Image.URL != nil && *part.Image.URL != "" {
						urlStr := *part.Image.URL
						// 如果是文件路径，需要读取文件并转换为base64
						if len(urlStr) > 0 && (urlStr[0] == '/' || urlStr[0] == '.') {
							// 这是本地文件路径，读取并转换
							data, err := os.ReadFile(urlStr)
							if err != nil {
								g.Log().Warningf(context.Background(), "Failed to read image file %s: %v, skipping", urlStr, err)
								continue
							}

							// 获取MIME类型
							mimeType := part.Image.MIMEType
							if mimeType == "" {
								ext := filepath.Ext(urlStr)
								mimeType = getMimeTypeForQwen(ext)
							}

							base64Data := base64.StdEncoding.EncodeToString(data)
							imageURL = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
						} else {
							// 假设是有效的HTTP URL或data URI
							imageURL = urlStr
						}
					}

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

			case schema.ChatMessagePartTypeAudioURL:
				// 通义千问可能不支持音频，作为文本描述
				contentParts = append(contentParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: "[音频文件]",
				})

			case schema.ChatMessagePartTypeVideoURL:
				// 通义千问可能不支持视频，作为文本描述
				contentParts = append(contentParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: "[视频文件]",
				})
			}
		}

		// 设置多模态内容数组
		openaiMsg.MultiContent = contentParts

	} else if len(msg.MultiContent) > 0 {
		// 使用旧版MultiContent字段
		var contentParts []openai.ChatMessagePart

		for _, part := range msg.MultiContent {
			switch part.Type {
			case schema.ChatMessagePartTypeText:
				contentParts = append(contentParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: part.Text,
				})

			case schema.ChatMessagePartTypeImageURL:
				if part.ImageURL != nil {
					imageURL := part.ImageURL.URL

					// 如果是文件路径，需要读取文件并转换为base64
					if len(imageURL) > 0 && (imageURL[0] == '/' || imageURL[0] == '.') {
						// 这是本地文件路径，读取并转换
						data, err := os.ReadFile(imageURL)
						if err != nil {
							g.Log().Warningf(context.Background(), "Failed to read image file %s: %v, skipping", imageURL, err)
							continue
						}

						// 获取MIME类型
						ext := filepath.Ext(imageURL)
						mimeType := getMimeTypeForQwen(ext)

						base64Data := base64.StdEncoding.EncodeToString(data)
						imageURL = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
					}

					contentParts = append(contentParts, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL:    imageURL,
							Detail: openai.ImageURLDetail(part.ImageURL.Detail),
						},
					})
				}
			}
		}

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

// MultimodalFile 多模态文件结构（避免导入common包）
type MultimodalFile struct {
	FileName     string
	FilePath     string
	RelativePath string
	FileType     string
}

const (
	FileTypeImage = "image"
	FileTypeAudio = "audio"
	FileTypeVideo = "video"
)

// BuildQwenMultimodalMessage 构建通义千问多模态消息
func BuildQwenMultimodalMessage(text string, files []*MultimodalFile) (*schema.Message, error) {
	if len(files) == 0 {
		return &schema.Message{
			Role:    schema.User,
			Content: text,
		}, nil
	}

	// 使用UserInputMultiContent字段
	var userInputParts []schema.MessageInputPart

	// 添加文本部分
	if text != "" {
		userInputParts = append(userInputParts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: text,
		})
	}

	// 添加文件部分
	for _, file := range files {
		part, err := buildQwenFilePart(file)
		if err != nil {
			g.Log().Errorf(nil, "Failed to build file part for %s: %v", file.FileName, err)
			continue
		}
		userInputParts = append(userInputParts, part)
	}

	message := &schema.Message{
		Role:                  schema.User,
		UserInputMultiContent: userInputParts,
	}

	return message, nil
}

// buildQwenFilePart 构建通义千问文件部分
func buildQwenFilePart(file *MultimodalFile) (schema.MessageInputPart, error) {
	switch file.FileType {
	case FileTypeImage:
		// 读取图片文件
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			return schema.MessageInputPart{}, fmt.Errorf("failed to read image file: %w", err)
		}

		// 获取MIME类型
		ext := filepath.Ext(file.FileName)
		mimeType := getMimeTypeForQwen(ext)

		// 编码为base64
		base64Data := base64.StdEncoding.EncodeToString(data)

		// 使用MessageInputPart，存储文件路径到URL，base64用于API调用
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL:        &file.FilePath, // 存储文件路径
					Base64Data: &base64Data,    // base64数据用于API调用
					MIMEType:   mimeType,
				},
			},
		}, nil

	default:
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: fmt.Sprintf("[文件: %s]", file.FileName),
		}, nil
	}
}

// getMimeTypeForQwen 获取MIME类型
func getMimeTypeForQwen(ext string) string {
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
