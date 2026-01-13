package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/os/gctx"

	coreErrors "github.com/Malowking/kbgo/core/errors"

	"github.com/Malowking/kbgo/core/formatter"
	coreModel "github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/history"
	"github.com/Malowking/kbgo/internal/logic/rewriter"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
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

// GetAnswer 使用指定模型生成答案（非流式）
func (x *Chat) GetAnswer(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, systemPrompt string, jsonFormat bool) (answer string, reasoningContent string, err error) {
	// 获取模型配置
	mc := coreModel.Registry.GetChatModel(modelID)
	if mc == nil {
		return "", "", coreErrors.Newf(coreErrors.ErrModelNotFound, "model not found: %s", modelID)
	}

	// 检查模型是否已启用，如果禁用则直接返回提示消息
	if !mc.Enabled {
		return "This model has been disabled", "", nil
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
	chatHistory, err := x.eh.GetHistory(convID, 50)
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

	// 捕获用户消息接收时间
	userMessageTime := time.Now()

	// 保存用户消息
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: question,
	}
	err = x.eh.SaveMessage(userMessage, convID, nil, &userMessageTime)
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

	// 准备响应格式
	var responseFormat *openai.ChatCompletionResponseFormat
	if jsonFormat {
		responseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	// 构建请求参数，直接使用模型配置中的参数
	chatParams := coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         getFloat32OrDefault(mc.Temperature, 0.7),
		MaxCompletionTokens: getIntOrDefault(mc.MaxCompletionTokens, 2000),
		TopP:                getFloat32OrDefault(mc.TopP, 0.9),
		FrequencyPenalty:    getFloat32OrDefault(mc.FrequencyPenalty, 0.0),
		PresencePenalty:     getFloat32OrDefault(mc.PresencePenalty, 0.0),
		N:                   getIntOrDefault(mc.N, 1),
		Stop:                mc.Stop,
		ResponseFormat:      responseFormat,
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
			return nil, coreErrors.New(coreErrors.ErrLLMCallFailed, "received empty choices from API")
		}

		return resp, nil
	})

	if err != nil {
		return "", "", coreErrors.Newf(coreErrors.ErrLLMCallFailed, "API调用失败: %v", err)
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
func (x *Chat) GetAnswerStream(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, systemPrompt string, jsonFormat bool) (answer schema.StreamReaderInterface[*schema.Message], err error) {
	// 获取模型配置
	mc := coreModel.Registry.GetChatModel(modelID)
	if mc == nil {
		return nil, coreErrors.Newf(coreErrors.ErrModelNotFound, "model not found: %s", modelID)
	}

	// 检查模型是否已启用，如果禁用则返回固定消息流
	if !mc.Enabled {
		// 创建 Pipe 用于流式传输
		streamReader, streamWriter := CreateStreamPipe(ctx, convID)

		// 发送禁用消息
		go func() {
			defer streamWriter.Close()

			// 发送固定消息
			streamWriter.Send(&schema.Message{
				Role:    schema.Assistant,
				Content: "This model has been disabled",
			}, nil)
		}()

		return streamReader, nil
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
	chatHistory, err := x.eh.GetHistory(convID, 50)
	if err != nil {
		return nil, err
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

	// 创建用户消息
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: question,
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

	// 检查历史中最后一条消息是否和当前用户消息重复
	// 如果重复，不添加；否则添加
	shouldAddUserMessage := true
	if len(chatHistory) > 0 {
		lastMsg := chatHistory[len(chatHistory)-1]
		if lastMsg.Role == schema.User && lastMsg.Content == question {
			g.Log().Warningf(ctx, "检测到重复的用户消息，跳过添加: %s", question)
			shouldAddUserMessage = false
		}
	}

	if shouldAddUserMessage {
		messages = append(messages, userMessage)
	}

	// 准备响应格式
	var responseFormat *openai.ChatCompletionResponseFormat
	if jsonFormat {
		responseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	// 构建请求参数，直接使用模型配置中的参数
	chatParams := coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         getFloat32OrDefault(mc.Temperature, 0.7),
		MaxCompletionTokens: getIntOrDefault(mc.MaxCompletionTokens, 2000),
		TopP:                getFloat32OrDefault(mc.TopP, 0.9),
		FrequencyPenalty:    getFloat32OrDefault(mc.FrequencyPenalty, 0.0),
		PresencePenalty:     getFloat32OrDefault(mc.PresencePenalty, 0.0),
		N:                   getIntOrDefault(mc.N, 1),
		Stop:                mc.Stop,
		ResponseFormat:      responseFormat,
	}

	// 记录开始时间
	start := time.Now()

	// 使用重试机制调用模型服务流式接口
	retryConfig := coreModel.DefaultSingleModelRetryConfig()
	streamResult, err := coreModel.RetryWithSameModel(ctx, mc.Name, retryConfig, func(ctx context.Context) (interface{}, error) {
		stream, err := modelService.ChatCompletionStream(ctx, chatParams)
		if err != nil {
			return nil, err
		}
		return stream, nil
	})

	if err != nil {
		return nil, coreErrors.Newf(coreErrors.ErrLLMCallFailed, "API调用失败: %v", err)
	}

	stream := streamResult.(*openai.ChatCompletionStream)

	// 创建 Pipe 用于流式传输
	// 优先使用 Redis Stream，Redis 不可用时回退到内存 channel
	streamReader, streamWriter := CreateStreamPipe(ctx, convID)

	// 保留原始 context 用于取消控制
	originalCtx := ctx
	// 使用 Background context 避免父 context 取消影响流式处理的完整性
	ctx = gctx.New()

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

// GenerateWithTools 使用指定模型进行工具调用（支持 Function Calling）
func (x *Chat) GenerateWithTools(ctx context.Context, modelID string, messages []*schema.Message, tools []*schema.ToolInfo) (*schema.Message, error) {
	// 获取模型配置
	mc := coreModel.Registry.GetChatModel(modelID)
	if mc == nil {
		return nil, coreErrors.Newf(coreErrors.ErrModelNotFound, "model not found: %s", modelID)
	}

	// 检查模型是否已启用，如果禁用则返回固定消息
	if !mc.Enabled {
		return &schema.Message{
			Role:    schema.Assistant,
			Content: "This model has been disabled",
		}, nil
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

	// 构建请求参数，直接使用模型配置中的参数
	chatParams := coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         getFloat32OrDefault(mc.Temperature, 0.7),
		MaxCompletionTokens: getIntOrDefault(mc.MaxCompletionTokens, 2000),
		TopP:                getFloat32OrDefault(mc.TopP, 0.9),
		FrequencyPenalty:    getFloat32OrDefault(mc.FrequencyPenalty, 0.0),
		PresencePenalty:     getFloat32OrDefault(mc.PresencePenalty, 0.0),
		N:                   getIntOrDefault(mc.N, 1),
		Stop:                mc.Stop,
		Tools:               openaiTools, // 添加工具列表
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

	// 使用重试机制调用模型服务
	retryConfig := coreModel.DefaultSingleModelRetryConfig()
	retryResult, err := coreModel.RetryWithSameModel(ctx, mc.Name, retryConfig, func(ctx context.Context) (interface{}, error) {
		resp, err := modelService.ChatCompletion(ctx, chatParams)
		if err != nil {
			return nil, err
		}

		if len(resp.Choices) == 0 {
			return nil, coreErrors.New(coreErrors.ErrLLMCallFailed, "received empty choices from API")
		}

		return resp, nil
	})

	if err != nil {
		g.Log().Errorf(ctx, "[GenerateWithTools] API调用失败: %v", err)
		return nil, coreErrors.Newf(coreErrors.ErrLLMCallFailed, "API调用失败: %v", err)
	}

	resp := retryResult.(*openai.ChatCompletionResponse)

	// 记录响应信息
	g.Log().Infof(ctx, "[GenerateWithTools] API响应 - Choices数: %d, Usage: %+v",
		len(resp.Choices), resp.Usage)

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
