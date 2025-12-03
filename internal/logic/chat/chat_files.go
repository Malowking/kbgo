package chat

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/formatter"
	coreModel "github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/history"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// GetAnswerWithFiles 统一的多模态对话处理（使用新架构）
func (x *Chat) GetAnswerWithFiles(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, files []*common.MultimodalFile) (answer string, err error) {
	// 获取模型配置
	mc := coreModel.Registry.Get(modelID)
	if mc == nil {
		return "", fmt.Errorf("model not found: %s", modelID)
	}

	// 根据模型类型选择格式适配器
	var msgFormatter formatter.MessageFormatter
	if IsQwenModel(mc.Name) {
		msgFormatter = formatter.NewQwenFormatter()
	} else {
		msgFormatter = formatter.NewOpenAIFormatter()
	}

	// 创建模型服务
	modelService := coreModel.NewModelService(mc.APIKey, mc.BaseURL, msgFormatter)

	// 获取聊天历史
	chatHistory, err := x.eh.GetHistory(convID, 100)
	if err != nil {
		return "", err
	}

	// 构建多模态消息
	userMessage, err := buildMultimodalMessage(question, files)
	if err != nil {
		return "", fmt.Errorf("构建多模态消息失败: %w", err)
	}

	// 保存用户消息
	err = x.eh.SaveMessage(userMessage, convID)
	if err != nil {
		return "", err
	}

	// 格式化文档为系统提示
	formattedDocs := formatDocumentsForQwen(docs)

	// 构建消息列表
	messages := []*schema.Message{
		{
			Role: schema.System,
			Content: "你是一个专业的AI助手，能够根据提供的参考信息准确回答用户问题。" +
				"如果没有提供参考信息，也请根据你的知识自由回答用户问题。\n\n" +
				formattedDocs,
		},
	}
	messages = append(messages, chatHistory...)
	messages = append(messages, userMessage)

	// 解析推理参数
	params := parseModelParams(mc.Extra)

	// 构建请求参数
	chatParams := coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         getFloat32OrDefault(params.Temperature, 0.7),
		MaxCompletionTokens: getIntOrDefault(params.MaxCompletionTokens, 2000),
		TopP:                getFloat32OrDefault(params.TopP, 0.9),
		FrequencyPenalty:    getFloat32OrDefault(params.FrequencyPenalty, 0.0),
		PresencePenalty:     getFloat32OrDefault(params.PresencePenalty, 0.0),
		N:                   getIntOrDefault(params.N, 1),
		Tools:               params.Tools,
		ToolChoice:          params.ToolChoice,
		ResponseFormat:      params.ResponseFormat,
	}

	// 记录开始时间
	start := time.Now()

	// 调用模型服务
	resp, err := modelService.ChatCompletion(ctx, chatParams)
	if err != nil {
		return "", fmt.Errorf("API调用失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("received empty choices from API")
	}

	answerContent := resp.Choices[0].Message.Content

	// 计算延迟
	latencyMs := time.Since(start).Milliseconds()

	// 创建assistant消息
	assistantMsg := &schema.Message{
		Role:    schema.Assistant,
		Content: answerContent,
	}

	// 创建带指标的消息
	msgWithMetrics := &history.MessageWithMetrics{
		Message:    assistantMsg,
		LatencyMs:  int(latencyMs),
		TokensUsed: resp.Usage.TotalTokens,
	}

	err = x.eh.SaveMessageWithMetrics(msgWithMetrics, convID)
	if err != nil {
		g.Log().Error(ctx, "save assistant message err: %v", err)
		return
	}

	return answerContent, nil
}

// GetAnswerStreamWithFiles 统一的多模态流式对话处理（使用新架构）
func (x *Chat) GetAnswerStreamWithFiles(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, files []*common.MultimodalFile) (answer *schema.StreamReader[*schema.Message], err error) {
	// 获取模型配置
	mc := coreModel.Registry.Get(modelID)
	if mc == nil {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	// 根据模型类型选择格式适配器
	var msgFormatter formatter.MessageFormatter
	if IsQwenModel(mc.Name) {
		msgFormatter = formatter.NewQwenFormatter()
	} else {
		msgFormatter = formatter.NewOpenAIFormatter()
	}

	// 创建模型服务
	modelService := coreModel.NewModelService(mc.APIKey, mc.BaseURL, msgFormatter)

	// 获取聊天历史
	chatHistory, err := x.eh.GetHistory(convID, 100)
	if err != nil {
		return nil, err
	}

	// 构建多模态消息
	userMessage, err := buildMultimodalMessage(question, files)
	if err != nil {
		return nil, fmt.Errorf("构建多模态消息失败: %w", err)
	}

	// 保存用户消息
	err = x.eh.SaveMessage(userMessage, convID)
	if err != nil {
		return nil, err
	}

	// 格式化文档为系统提示
	formattedDocs := formatDocumentsForQwen(docs)

	// 构建消息列表
	messages := []*schema.Message{
		{
			Role: schema.System,
			Content: "你是一个专业的AI助手，能够根据提供的参考信息准确回答用户问题。" +
				"如果没有提供参考信息，也请根据你的知识自由回答用户问题。\n\n" +
				formattedDocs,
		},
	}
	messages = append(messages, chatHistory...)
	messages = append(messages, userMessage)

	// 解析推理参数
	params := parseModelParams(mc.Extra)

	// 构建请求参数
	chatParams := coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         getFloat32OrDefault(params.Temperature, 0.7),
		MaxCompletionTokens: getIntOrDefault(params.MaxCompletionTokens, 2000),
		TopP:                getFloat32OrDefault(params.TopP, 0.9),
		FrequencyPenalty:    getFloat32OrDefault(params.FrequencyPenalty, 0.0),
		PresencePenalty:     getFloat32OrDefault(params.PresencePenalty, 0.0),
		N:                   getIntOrDefault(params.N, 1),
		Tools:               params.Tools,
		ToolChoice:          params.ToolChoice,
		ResponseFormat:      params.ResponseFormat,
	}

	// 记录开始时间
	start := time.Now()

	// 调用模型服务流式接口
	stream, err := modelService.ChatCompletionStream(ctx, chatParams)
	if err != nil {
		return nil, fmt.Errorf("API调用失败: %w", err)
	}

	// 创建 Pipe 用于流式传输
	streamReader, streamWriter := schema.Pipe[*schema.Message](10)

	// 启动goroutine处理流式响应
	go func() {
		defer streamWriter.Close()
		defer stream.Close()

		var fullContent strings.Builder
		var tokenCount int

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// 流结束，保存完整消息
				assistantMsg := &schema.Message{
					Role:    schema.Assistant,
					Content: fullContent.String(),
				}

				// 计算延迟
				latencyMs := time.Since(start).Milliseconds()

				// 创建带指标的消息
				msgWithMetrics := &history.MessageWithMetrics{
					Message:    assistantMsg,
					LatencyMs:  int(latencyMs),
					TokensUsed: tokenCount,
				}

				// 异步保存消息
				saveErr := x.eh.SaveMessageWithMetrics(msgWithMetrics, convID)
				if saveErr != nil {
					g.Log().Errorf(ctx, "save assistant message err: %v", saveErr)
				}

				return
			}

			if err != nil {
				g.Log().Errorf(ctx, "stream receive error: %v", err)
				streamWriter.Send(&schema.Message{
					Role:    schema.Assistant,
					Content: "",
				}, err)
				return
			}

			// 处理流式响应
			if len(response.Choices) > 0 {
				delta := response.Choices[0].Delta.Content
				if delta != "" {
					fullContent.WriteString(delta)

					// 创建增量消息并发送到流
					chunk := &schema.Message{
						Role:    schema.Assistant,
						Content: delta,
					}
					closed := streamWriter.Send(chunk, nil)
					if closed {
						g.Log().Warningf(ctx, "stream writer closed unexpectedly")
						return
					}
				}

				// 累计token数量（如果有usage信息）
				if response.Usage != nil {
					tokenCount = response.Usage.TotalTokens
				}
			}
		}
	}()

	return streamReader, nil
}

// formatDocumentsForQwen 格式化文档为可读的字符串
func formatDocumentsForQwen(docs []*schema.Document) string {
	if len(docs) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("参考资料:\n")
	for i, doc := range docs {
		builder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, doc.Content))
	}
	return builder.String()
}

// IsQwenModel 判断是否为Qwen模型
func IsQwenModel(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), "qwen")
}

// buildMultimodalMessage 构建多模态消息
func buildMultimodalMessage(text string, files []*common.MultimodalFile) (*schema.Message, error) {
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
		part, err := buildFilePart(file)
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

// buildFilePart 构建文件部分
func buildFilePart(file *common.MultimodalFile) (schema.MessageInputPart, error) {
	switch file.FileType {
	case common.FileTypeImage:
		// 读取图片文件
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			return schema.MessageInputPart{}, fmt.Errorf("failed to read image file: %w", err)
		}

		// 获取MIME类型
		ext := filepath.Ext(file.FileName)
		mimeType := getMimeTypeForFile(ext)

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

// getMimeTypeForFile 获取MIME类型
func getMimeTypeForFile(ext string) string {
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

// 辅助函数
func getFloat32OrDefault(val *float32, defaultVal float32) float32 {
	if val != nil {
		return *val
	}
	return defaultVal
}

func getIntOrDefault(val *int, defaultVal int) int {
	if val != nil {
		return *val
	}
	return defaultVal
}
