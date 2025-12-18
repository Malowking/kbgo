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
	"github.com/Malowking/kbgo/internal/logic/rewriter"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/sashabaranov/go-openai"
)

var chatInstance *Chat

type Chat struct {
	eh            *history.Manager
	queryRewriter *rewriter.QueryRewriter
}

func GetChat() *Chat {
	return chatInstance
}

// InitHistory 初始化历史管理器
func InitHistory() {
	ctx := gctx.New()
	g.Log().Info(ctx, "Initializing Chat history manager...")

	chatInstance = &Chat{
		eh:            history.NewManager(),
		queryRewriter: rewriter.NewQueryRewriter(),
	}

	g.Log().Info(ctx, "Chat history manager initialized successfully")
}

// parseModelParams 从 Extra 字段解析推理参数
func parseModelParams(extra map[string]any) *ModelParams {
	params := GetDefaultParams()

	if extra == nil {
		return &params
	}

	// 解析各个参数
	if temp, ok := extra["temperature"].(float64); ok {
		params.Temperature = ToPointer(float32(temp))
	}
	if topP, ok := extra["topP"].(float64); ok {
		params.TopP = ToPointer(float32(topP))
	}
	if maxCompletionTokens, ok := extra["maxCompletionTokens"].(int); ok {
		params.MaxCompletionTokens = ToPointer(maxCompletionTokens)
	}
	if freqPenalty, ok := extra["frequencyPenalty"].(float64); ok {
		params.FrequencyPenalty = ToPointer(float32(freqPenalty))
	}
	if presPenalty, ok := extra["presencePenalty"].(float64); ok {
		params.PresencePenalty = ToPointer(float32(presPenalty))
	}
	if n, ok := extra["n"].(int); ok {
		params.N = ToPointer(n)
	}
	if stop, ok := extra["stop"].([]interface{}); ok {
		stopWords := make([]string, 0, len(stop))
		for _, s := range stop {
			if str, ok := s.(string); ok {
				stopWords = append(stopWords, str)
			}
		}
		if len(stopWords) > 0 {
			params.Stop = stopWords
		}
	}

	return &params
}

// GetAnswer 使用指定模型生成答案（非流式）
func (x *Chat) GetAnswer(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, systemPrompt string, jsonFormat bool) (answer string, reasoningContent string, err error) {
	// 获取模型配置
	mc := coreModel.Registry.Get(modelID)
	if mc == nil {
		return "", "", fmt.Errorf("model not found: %s", modelID)
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
		return "", "", err
	}

	// 使用查询重写器进行指代消解
	rewriteConfig := rewriter.DefaultConfig()
	rewriteConfig.ModelID = modelID // 使用相同的模型进行重写
	rewrittenQuestion, err := x.queryRewriter.RewriteQuery(ctx, question, chatHistory, rewriteConfig)
	if err != nil {
		g.Log().Warningf(ctx, "查询重写失败: %v，使用原查询", err)
		rewrittenQuestion = question
	}
	_ = rewrittenQuestion

	// 保存用户消息（使用原始问题）
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: question,
	}
	err = x.eh.SaveMessage(userMessage, convID)
	if err != nil {
		return "", "", err
	}

	// 格式化文档为系统提示
	formattedDocs := formatDocumentsForChat(docs)

	// 构建系统提示词
	var systemContent string
	if systemPrompt != "" {
		// 如果提供了自定义系统提示词，使用它
		systemContent = systemPrompt
		// 如果有检索到的文档，追加到系统提示词后面
		if formattedDocs != "" {
			systemContent += "\n\n" + formattedDocs
		}
	} else {
		// 使用默认系统提示词
		systemContent = "你是一个专业的AI助手，能够根据提供的参考信息准确回答用户问题。" +
			"如果没有提供参考信息，也请根据你的知识自由回答用户问题。\n\n" +
			formattedDocs
	}

	// 构建消息列表
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemContent,
		},
	}
	messages = append(messages, chatHistory...)
	messages = append(messages, userMessage)

	// 解析推理参数
	params := parseModelParams(mc.Extra)

	// 如果需要JSON格式化，设置ResponseFormat
	if jsonFormat {
		params.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

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
		Stop:                params.Stop,
		Tools:               params.Tools,
		ToolChoice:          params.ToolChoice,
		ResponseFormat:      params.ResponseFormat,
	}

	// 记录开始时间
	start := time.Now()

	// 使用重试机制调用模型服务
	retryConfig := coreModel.DefaultSingleModelRetryConfig()
	result, err := coreModel.RetryWithSameModel(ctx, mc.Name, retryConfig, func(ctx context.Context) (interface{}, error) {
		resp, err := modelService.ChatCompletion(ctx, chatParams)
		if err != nil {
			return nil, err
		}

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("received empty choices from API")
		}

		return resp, nil
	})

	if err != nil {
		return "", "", fmt.Errorf("API调用失败: %w", err)
	}

	resp := result.(*openai.ChatCompletionResponse)
	answerContent := resp.Choices[0].Message.Content
	thinkContent := resp.Choices[0].Message.ReasoningContent

	// 计算延迟
	latencyMs := time.Since(start).Milliseconds()

	// 创建assistant消息
	assistantMsg := &schema.Message{
		Role:    schema.Assistant,
		Content: answerContent,
	}

	// 只有当思考内容不为空时才添加到消息中
	if thinkContent != "" {
		assistantMsg.ReasoningContent = thinkContent
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
		return "", "", err
	}

	return answerContent, thinkContent, nil
}

// GetAnswerStream 使用指定模型流式生成答案
func (x *Chat) GetAnswerStream(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, systemPrompt string, jsonFormat bool) (answer *schema.StreamReader[*schema.Message], err error) {
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

	// 使用查询重写器进行指代消解（如果需要）
	rewriteConfig := rewriter.DefaultConfig()
	rewriteConfig.ModelID = modelID // 使用相同的模型进行重写
	rewrittenQuestion, err := x.queryRewriter.RewriteQuery(ctx, question, chatHistory, rewriteConfig)
	if err != nil {
		g.Log().Warningf(ctx, "查询重写失败: %v，使用原查询", err)
		rewrittenQuestion = question
	}
	// TODO: rewrittenQuestion 可用于文档检索优化，当前版本文档已预先检索
	_ = rewrittenQuestion

	// 保存用户消息（使用原始问题）
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: question,
	}
	err = x.eh.SaveMessage(userMessage, convID)
	if err != nil {
		return nil, err
	}

	// 格式化文档为系统提示
	formattedDocs := formatDocumentsForChat(docs)

	// 构建系统提示词
	var systemContent string
	if systemPrompt != "" {
		// 如果提供了自定义系统提示词，使用它
		systemContent = systemPrompt
		// 如果有检索到的文档，追加到系统提示词后面
		if formattedDocs != "" {
			systemContent += "\n\n" + formattedDocs
		}
	} else {
		// 使用默认系统提示词
		systemContent = "你是一个专业的AI助手，能够根据提供的参考信息准确回答用户问题。" +
			"如果没有提供参考信息，也请根据你的知识自由回答用户问题。\n\n" +
			formattedDocs
	}

	// 构建消息列表
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemContent,
		},
	}
	messages = append(messages, chatHistory...)
	messages = append(messages, userMessage)

	// 解析推理参数
	params := parseModelParams(mc.Extra)

	// 如果需要JSON格式化，设置ResponseFormat
	if jsonFormat {
		params.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

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
		Stop:                params.Stop,
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

	// 保留原始 context 用于取消控制
	originalCtx := ctx
	// 使用 Background context 避免父 context 取消影响流式处理的完整性
	ctx = context.Background()

	// 启动goroutine处理流式响应
	go func() {
		defer streamWriter.Close()
		defer stream.Close()

		var fullContent strings.Builder
		var tokenCount int

		var fullReasoningContent strings.Builder

		for {
			// 检查客户端是否断开连接
			select {
			case <-originalCtx.Done():
				g.Log().Warning(ctx, "Stream cancelled by client, stopping goroutine")
				return
			default:
			}

			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// 流结束，保存完整消息
				assistantMsg := &schema.Message{
					Role:    schema.Assistant,
					Content: fullContent.String(),
				}

				// 只有当思考内容不为空时才添加到消息中
				if fullReasoningContent.Len() > 0 {
					assistantMsg.ReasoningContent = fullReasoningContent.String()
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
				rdelta := response.Choices[0].Delta.ReasoningContent

				// 创建增量消息
				chunk := &schema.Message{
					Role: schema.Assistant,
				}

				// 处理普通内容
				if delta != "" {
					fullContent.WriteString(delta)
					chunk.Content = delta
				}

				// 处理思考内容
				if rdelta != "" {
					fullReasoningContent.WriteString(rdelta)
					chunk.ReasoningContent = rdelta
				}

				// 只有当有内容或思考内容时才发送
				if delta != "" || rdelta != "" {
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

// preprocessMultimodalMessages 预处理多模态消息，将文件路径转换为base64
func preprocessMultimodalMessages(ctx context.Context, messages []*schema.Message) error {
	for _, msg := range messages {
		// 处理 UserInputMultiContent
		if len(msg.UserInputMultiContent) > 0 {
			for i := range msg.UserInputMultiContent {
				part := &msg.UserInputMultiContent[i]

				// 处理图片
				if part.Type == schema.ChatMessagePartTypeImageURL && part.Image != nil {
					// 如果已经有base64数据，跳过
					if part.Image.Base64Data != nil && *part.Image.Base64Data != "" {
						continue
					}

					// 如果URL是文件路径，读取并转换为base64
					if part.Image.URL != nil && *part.Image.URL != "" {
						urlStr := *part.Image.URL
						if len(urlStr) > 0 && (urlStr[0] == '/' || urlStr[0] == '.') {
							data, err := os.ReadFile(urlStr)
							if err != nil {
								g.Log().Warningf(ctx, "Failed to read image file %s: %v, skipping", urlStr, err)
								continue
							}

							// 获取MIME类型
							mimeType := part.Image.MIMEType
							if mimeType == "" {
								ext := filepath.Ext(urlStr)
								mimeType = getMimeType(ext)
							}

							base64Data := base64.StdEncoding.EncodeToString(data)
							// 构造data URI格式
							dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
							part.Image.URL = &dataURI
						}
					}
				}

				// 处理音频
				if part.Type == schema.ChatMessagePartTypeAudioURL && part.Audio != nil {
					// 如果已经有base64数据，跳过
					if part.Audio.Base64Data != nil && *part.Audio.Base64Data != "" {
						continue
					}

					// 如果URL是文件路径，读取并转换为base64
					if part.Audio.URL != nil && *part.Audio.URL != "" {
						urlStr := *part.Audio.URL
						if len(urlStr) > 0 && (urlStr[0] == '/' || urlStr[0] == '.') {
							data, err := os.ReadFile(urlStr)
							if err != nil {
								g.Log().Warningf(ctx, "Failed to read audio file %s: %v, skipping", urlStr, err)
								continue
							}

							// 获取MIME类型
							mimeType := part.Audio.MIMEType
							if mimeType == "" {
								ext := filepath.Ext(urlStr)
								mimeType = getMimeType(ext)
							}

							base64Data := base64.StdEncoding.EncodeToString(data)
							// 构造data URI格式
							dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
							part.Audio.URL = &dataURI
						}
					}
				}

				// 处理视频
				if part.Type == schema.ChatMessagePartTypeVideoURL && part.Video != nil {
					// 如果已经有base64数据，跳过
					if part.Video.Base64Data != nil && *part.Video.Base64Data != "" {
						continue
					}

					// 如果URL是文件路径，读取并转换为base64
					if part.Video.URL != nil && *part.Video.URL != "" {
						urlStr := *part.Video.URL
						if len(urlStr) > 0 && (urlStr[0] == '/' || urlStr[0] == '.') {
							data, err := os.ReadFile(urlStr)
							if err != nil {
								g.Log().Warningf(ctx, "Failed to read video file %s: %v, skipping", urlStr, err)
								continue
							}

							// 获取MIME类型
							mimeType := part.Video.MIMEType
							if mimeType == "" {
								ext := filepath.Ext(urlStr)
								mimeType = getMimeType(ext)
							}

							base64Data := base64.StdEncoding.EncodeToString(data)
							// 构造data URI格式
							dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
							part.Video.URL = &dataURI
						}
					}
				}
			}
		}

		// 处理 MultiContent（旧版字段）
		if len(msg.MultiContent) > 0 {
			for i := range msg.MultiContent {
				part := &msg.MultiContent[i]

				// 处理图片
				if part.Type == schema.ChatMessagePartTypeImageURL && part.ImageURL != nil {
					urlStr := part.ImageURL.URL
					// 如果是文件路径，读取并转换为base64
					if len(urlStr) > 0 && (urlStr[0] == '/' || urlStr[0] == '.') {
						data, err := os.ReadFile(urlStr)
						if err != nil {
							g.Log().Warningf(ctx, "Failed to read image file %s: %v, skipping", urlStr, err)
							continue
						}

						// 获取MIME类型
						ext := filepath.Ext(urlStr)
						mimeType := getMimeType(ext)

						base64Data := base64.StdEncoding.EncodeToString(data)
						// 构造data URI格式
						part.ImageURL.URL = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
					}
				}

				// 处理音频
				if part.Type == schema.ChatMessagePartTypeAudioURL && part.AudioURL != nil {
					urlStr := part.AudioURL.URL
					// 如果是文件路径，读取并转换为base64
					if len(urlStr) > 0 && (urlStr[0] == '/' || urlStr[0] == '.') {
						data, err := os.ReadFile(urlStr)
						if err != nil {
							g.Log().Warningf(ctx, "Failed to read audio file %s: %v, skipping", urlStr, err)
							continue
						}

						// 获取MIME类型
						ext := filepath.Ext(urlStr)
						mimeType := getMimeType(ext)

						base64Data := base64.StdEncoding.EncodeToString(data)
						// 构造data URI格式
						part.AudioURL.URL = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
					}
				}

				// 处理视频
				if part.Type == schema.ChatMessagePartTypeVideoURL && part.VideoURL != nil {
					urlStr := part.VideoURL.URL
					// 如果是文件路径，读取并转换为base64
					if len(urlStr) > 0 && (urlStr[0] == '/' || urlStr[0] == '.') {
						data, err := os.ReadFile(urlStr)
						if err != nil {
							g.Log().Warningf(ctx, "Failed to read video file %s: %v, skipping", urlStr, err)
							continue
						}

						// 获取MIME类型
						ext := filepath.Ext(urlStr)
						mimeType := getMimeType(ext)

						base64Data := base64.StdEncoding.EncodeToString(data)
						// 构造data URI格式
						part.VideoURL.URL = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
					}
				}
			}
		}
	}
	return nil
}

// getMimeType 根据文件扩展名获取MIME类型
// 已废弃：使用 common.GetMimeType 替代
func getMimeType(ext string) string {
	return common.GetMimeType(ext)
}

// GenerateWithTools 使用指定模型进行工具调用（支持 Function Calling）
func (x *Chat) GenerateWithTools(ctx context.Context, modelID string, messages []*schema.Message, tools []*schema.ToolInfo) (*schema.Message, error) {
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

	// 解析推理参数
	params := parseModelParams(mc.Extra)

	// 转换 schema.ToolInfo 到 openai.Tool
	var openaiTools []openai.Tool
	if len(tools) > 0 {
		for _, tool := range tools {
			// 将ParamsOneOf转换为OpenAPIV3格式
			var toolParams interface{}
			if tool.ParamsOneOf != nil {
				openAPIV3Schema, err := tool.ParamsOneOf.ToOpenAPIV3()
				if err != nil {
					g.Log().Warningf(ctx, "Failed to convert tool params to OpenAPIV3: %v", err)
					continue
				}
				toolParams = openAPIV3Schema
			}

			openaiTools = append(openaiTools, openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        tool.Name,
					Description: tool.Desc,
					Parameters:  toolParams,
				},
			})
		}
	}

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
		Stop:                params.Stop,
		Tools:               openaiTools, // 添加工具列表
		ResponseFormat:      params.ResponseFormat,
	}

	// 只有在有工具时才设置 ToolChoice
	if len(openaiTools) > 0 {
		chatParams.ToolChoice = "auto" // 让模型自动决定是否调用工具
	}

	// 记录开始时间
	start := time.Now()

	// 记录请求信息
	g.Log().Infof(ctx, "[GenerateWithTools] 调用模型: %s, 消息数: %d, 工具数: %d",
		mc.Name, len(messages), len(openaiTools))

	// 打印最后一条消息的内容（用于调试）
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		g.Log().Debugf(ctx, "[GenerateWithTools] 最后一条消息 - Role: %s, Content长度: %d, ToolCalls数: %d",
			lastMsg.Role, len(lastMsg.Content), len(lastMsg.ToolCalls))
	}

	// 调用模型服务
	resp, err := modelService.ChatCompletion(ctx, chatParams)
	if err != nil {
		g.Log().Errorf(ctx, "[GenerateWithTools] API调用失败: %v", err)
		return nil, fmt.Errorf("API调用失败: %w", err)
	}

	// 记录响应信息
	g.Log().Infof(ctx, "[GenerateWithTools] API响应 - Choices数: %d, Usage: %+v",
		len(resp.Choices), resp.Usage)

	if len(resp.Choices) == 0 {
		// 打印完整的响应以便调试
		g.Log().Errorf(ctx, "[GenerateWithTools] 收到空的Choices! 完整响应: ID=%s, Model=%s, Object=%s",
			resp.ID, resp.Model, resp.Object)
		return nil, fmt.Errorf("received empty choices from API")
	}

	// 计算延迟
	latencyMs := time.Since(start).Milliseconds()

	// 转换 OpenAI 响应为 schema.Message
	choice := resp.Choices[0]
	result := &schema.Message{
		Role:    schema.Assistant,
		Content: choice.Message.Content,
	}

	// 只有当思考内容不为空时才添加到消息中
	if choice.Message.ReasoningContent != "" {
		result.ReasoningContent = choice.Message.ReasoningContent
	}

	// 转换 ToolCalls
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]schema.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = schema.ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type), // Convert openai.ToolType to string
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	// 添加指标信息到返回的消息中（通过扩展字段）
	result.Extra = map[string]any{
		"latency_ms":  latencyMs,
		"tokens_used": resp.Usage.TotalTokens,
	}

	return result, nil
}

// SaveMessageWithMetadata 保存带元数据的消息
func (x *Chat) SaveMessageWithMetadata(message *schema.Message, convID string, metadata map[string]interface{}) error {
	return x.eh.SaveMessageWithMetadata(message, convID, metadata)
}

// SaveStreamingMessageWithMetadata 保存流式传输的完整消息和元数据
func (x *Chat) SaveStreamingMessageWithMetadata(convID string, content string, metadata map[string]interface{}) error {
	message := &schema.Message{
		Role:    schema.Assistant,
		Content: content,
	}
	return x.eh.SaveMessageWithMetadata(message, convID, metadata)
}

// formatDocumentsForChat 格式化文档为聊天上下文
func formatDocumentsForChat(docs []*schema.Document) string {
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
