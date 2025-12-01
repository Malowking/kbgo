package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/common"
	coreModel "github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/history"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// GetAnswerWithFilesUsingQwen 使用QwenAdapter处理多模态对话
func (x *Chat) GetAnswerWithFilesUsingQwen(ctx context.Context, mc *coreModel.ModelConfig, convID string, docs []*schema.Document, question string, files []*common.MultimodalFile) (answer string, err error) {
	// 创建QwenAdapter
	adapter := coreModel.NewQwenAdapter(mc.APIKey, mc.BaseURL)

	// 获取聊天历史
	chatHistory, err := x.eh.GetHistory(convID, 100)
	if err != nil {
		return "", err
	}

	// 转换common.MultimodalFile到model.MultimodalFile
	modelFiles := make([]*coreModel.MultimodalFile, len(files))
	for i, file := range files {
		modelFiles[i] = &coreModel.MultimodalFile{
			FileName:     file.FileName,
			FilePath:     file.FilePath,
			RelativePath: file.RelativePath,
			FileType:     string(file.FileType),
		}
	}

	// 使用QwenAdapter构建多模态消息
	userMessage, err := coreModel.BuildQwenMultimodalMessage(question, modelFiles)
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
	temperature := 0.7
	maxTokens := 2000
	if params.Temperature != nil {
		temperature = *params.Temperature
	}
	if params.MaxTokens != nil {
		maxTokens = *params.MaxTokens
	}

	// 记录开始时间
	start := time.Now()

	// 调用QwenAdapter
	resp, err := adapter.ChatCompletion(ctx, mc.Name, messages, temperature, maxTokens)
	if err != nil {
		return "", fmt.Errorf("Qwen API调用失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("received empty choices from Qwen API")
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

// GetAnswerStreamWithFilesUsingQwen 使用QwenAdapter处理多模态流式对话
func (x *Chat) GetAnswerStreamWithFilesUsingQwen(ctx context.Context, mc *coreModel.ModelConfig, convID string, docs []*schema.Document, question string, files []*common.MultimodalFile) (answer *schema.StreamReader[*schema.Message], err error) {
	// 创建QwenAdapter
	adapter := coreModel.NewQwenAdapter(mc.APIKey, mc.BaseURL)

	// 获取聊天历史
	chatHistory, err := x.eh.GetHistory(convID, 100)
	if err != nil {
		return nil, err
	}

	// 转换common.MultimodalFile到model.MultimodalFile
	modelFiles := make([]*coreModel.MultimodalFile, len(files))
	for i, file := range files {
		modelFiles[i] = &coreModel.MultimodalFile{
			FileName:     file.FileName,
			FilePath:     file.FilePath,
			RelativePath: file.RelativePath,
			FileType:     string(file.FileType),
		}
	}

	// 使用QwenAdapter构建多模态消息
	userMessage, err := coreModel.BuildQwenMultimodalMessage(question, modelFiles)
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
	temperature := 0.7
	maxTokens := 2000
	if params.Temperature != nil {
		temperature = *params.Temperature
	}
	if params.MaxTokens != nil {
		maxTokens = *params.MaxTokens
	}

	// 记录开始时间
	start := time.Now()

	// 调用QwenAdapter流式接口
	stream, err := adapter.ChatCompletionStream(ctx, mc.Name, messages, temperature, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("Qwen API调用失败: %w", err)
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

// formatDocumentsForQwen 格式化文档为Qwen可读的字符串
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
