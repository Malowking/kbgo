package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Malowking/kbgo/core/common"
	coreModel "github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/history"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

var chatInstance *Chat

type Chat struct {
	eh *history.Manager
}

func GetChat() *Chat {
	return chatInstance
}

// InitHistory 初始化历史管理器
func InitHistory() {
	ctx := gctx.New()
	g.Log().Info(ctx, "Initializing Chat history manager...")

	chatInstance = &Chat{
		eh: history.NewManager(),
	}

	g.Log().Info(ctx, "Chat history manager initialized successfully")
}

// getModelAndParams 根据 model_id 获取模型客户端和推理参数
func (x *Chat) getModelAndParams(ctx context.Context, modelID string) (model.BaseChatModel, *ModelParams, error) {
	// 从 Registry 获取模型配置
	mc := coreModel.Registry.Get(modelID)
	if mc == nil {
		return nil, nil, fmt.Errorf("model not found: %s", modelID)
	}

	// 确保是 LLM 类型的模型
	if mc.Type != coreModel.ModelTypeLLM && mc.Type != coreModel.ModelTypeMultimodal {
		return nil, nil, fmt.Errorf("model %s is not a chat model, type: %s", modelID, mc.Type)
	}

	// 创建 EINO ChatModel
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  mc.APIKey,
		BaseURL: mc.BaseURL,
		Model:   mc.Name,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	// 从 Extra 字段解析推理参数
	params := parseModelParams(mc.Extra)

	return chatModel, params, nil
}

// parseModelParams 从 Extra 字段解析推理参数
func parseModelParams(extra map[string]any) *ModelParams {
	params := GetDefaultParams()

	if extra == nil {
		return &params
	}

	// 解析各个参数
	if temp, ok := extra["temperature"].(float64); ok {
		params.Temperature = ToPointer(temp)
	}
	if topP, ok := extra["topP"].(float64); ok {
		params.TopP = ToPointer(topP)
	}
	if maxTokens, ok := extra["maxTokens"].(float64); ok {
		params.MaxTokens = ToPointer(int(maxTokens))
	}
	if freqPenalty, ok := extra["frequencyPenalty"].(float64); ok {
		params.FrequencyPenalty = ToPointer(freqPenalty)
	}
	if presPenalty, ok := extra["presencePenalty"].(float64); ok {
		params.PresencePenalty = ToPointer(presPenalty)
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
func (x *Chat) GetAnswer(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string) (answer string, err error) {
	// 获取模型和参数
	chatModel, params, err := x.getModelAndParams(ctx, modelID)
	if err != nil {
		return "", err
	}

	messages, err := x.docsMessages(ctx, convID, docs, question)
	if err != nil {
		return "", err
	}

	// 记录开始时间
	start := time.Now()

	// 生成答案
	opts := params.ToModelOptions()
	result, err := chatModel.Generate(ctx, messages, opts...)
	if err != nil {
		return "", fmt.Errorf("生成答案失败: %w", err)
	}

	// 计算延迟
	latencyMs := time.Since(start).Milliseconds()

	// 获取token使用量
	tokensUsed := 0
	if result.ResponseMeta != nil && result.ResponseMeta.Usage != nil {
		tokensUsed = result.ResponseMeta.Usage.TotalTokens
	}

	// 创建带指标的消息
	msgWithMetrics := &history.MessageWithMetrics{
		Message:    result,
		LatencyMs:  int(latencyMs),
		TokensUsed: tokensUsed,
	}

	err = x.eh.SaveMessageWithMetrics(msgWithMetrics, convID)
	if err != nil {
		g.Log().Error(ctx, "save assistant message err: %v", err)
		return
	}
	return result.Content, nil
}

// GetAnswerStream 使用指定模型流式生成答案
func (x *Chat) GetAnswerStream(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string) (answer *schema.StreamReader[*schema.Message], err error) {
	// 获取模型和参数
	chatModel, params, err := x.getModelAndParams(ctx, modelID)
	if err != nil {
		return nil, err
	}

	messages, err := x.docsMessages(ctx, convID, docs, question)
	if err != nil {
		return nil, err
	}

	// 记录开始时间
	start := time.Now()

	// 流式生成
	ctx = context.Background()
	opts := params.ToModelOptions()
	streamData, err := chatModel.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("生成答案失败: %w", err)
	}

	srs := streamData.Copy(2)
	go func() {
		// for save
		fullMsgs := make([]*schema.Message, 0)
		defer func() {
			srs[1].Close()
			fullMsg, err := schema.ConcatMessages(fullMsgs)
			if err != nil {
				g.Log().Error(ctx, "error concatenating messages: %v", err)
				return
			}

			// 计算延迟
			latencyMs := time.Since(start).Milliseconds()

			// 获取token使用量
			tokensUsed := 0
			if fullMsg.ResponseMeta != nil && fullMsg.ResponseMeta.Usage != nil {
				tokensUsed = fullMsg.ResponseMeta.Usage.TotalTokens
			}

			// 创建带指标的消息
			msgWithMetrics := &history.MessageWithMetrics{
				Message:    fullMsg,
				LatencyMs:  int(latencyMs),
				TokensUsed: tokensUsed,
			}

			err = x.eh.SaveMessageWithMetrics(msgWithMetrics, convID)
			if err != nil {
				g.Log().Error(ctx, "save assistant message err: %v", err)
				return
			}
		}()
	outer:
		for {
			select {
			case <-ctx.Done():
				fmt.Println("context done", ctx.Err())
				return
			default:
				chunk, err := srs[1].Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break outer
					}
				}
				fullMsgs = append(fullMsgs, chunk)
			}
		}
	}()

	return srs[0], nil
}

// GetAnswerWithFiles 使用指定模型进行多模态对话
func (x *Chat) GetAnswerWithFiles(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, files []*common.MultimodalFile) (answer string, err error) {
	// 获取模型和参数
	chatModel, params, err := x.getModelAndParams(ctx, modelID)
	if err != nil {
		return "", err
	}

	messages, err := x.docsMessagesWithFiles(ctx, convID, docs, question, files)
	if err != nil {
		return "", err
	}

	// 记录开始时间
	start := time.Now()

	// 生成答案
	opts := params.ToModelOptions()
	result, err := chatModel.Generate(ctx, messages, opts...)
	if err != nil {
		return "", fmt.Errorf("生成答案失败: %w", err)
	}

	// 计算延迟
	latencyMs := time.Since(start).Milliseconds()

	// 获取token使用量
	tokensUsed := 0
	if result.ResponseMeta != nil && result.ResponseMeta.Usage != nil {
		tokensUsed = result.ResponseMeta.Usage.TotalTokens
	}

	// 创建带指标的消息
	msgWithMetrics := &history.MessageWithMetrics{
		Message:    result,
		LatencyMs:  int(latencyMs),
		TokensUsed: tokensUsed,
	}

	err = x.eh.SaveMessageWithMetrics(msgWithMetrics, convID)
	if err != nil {
		g.Log().Error(ctx, "save assistant message err: %v", err)
		return
	}
	return result.Content, nil
}

// GetAnswerStreamWithFiles 使用指定模型进行多模态流式对话
func (x *Chat) GetAnswerStreamWithFiles(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, files []*common.MultimodalFile) (answer *schema.StreamReader[*schema.Message], err error) {
	// 获取模型和参数
	chatModel, params, err := x.getModelAndParams(ctx, modelID)
	if err != nil {
		return nil, err
	}

	messages, err := x.docsMessagesWithFiles(ctx, convID, docs, question, files)
	if err != nil {
		return nil, err
	}

	// 记录开始时间
	start := time.Now()

	ctx = context.Background()
	opts := params.ToModelOptions()
	streamData, err := chatModel.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("生成答案失败: %w", err)
	}

	srs := streamData.Copy(2)
	go func() {
		// for save
		fullMsgs := make([]*schema.Message, 0)
		defer func() {
			srs[1].Close()
			fullMsg, err := schema.ConcatMessages(fullMsgs)
			if err != nil {
				g.Log().Error(ctx, "error concatenating messages: %v", err)
				return
			}

			// 计算延迟
			latencyMs := time.Since(start).Milliseconds()

			// 获取token使用量
			tokensUsed := 0
			if fullMsg.ResponseMeta != nil && fullMsg.ResponseMeta.Usage != nil {
				tokensUsed = fullMsg.ResponseMeta.Usage.TotalTokens
			}

			// 创建带指标的消息
			msgWithMetrics := &history.MessageWithMetrics{
				Message:    fullMsg,
				LatencyMs:  int(latencyMs),
				TokensUsed: tokensUsed,
			}

			err = x.eh.SaveMessageWithMetrics(msgWithMetrics, convID)
			if err != nil {
				g.Log().Error(ctx, "save assistant message err: %v", err)
				return
			}
		}()
	outer:
		for {
			select {
			case <-ctx.Done():
				fmt.Println("context done", ctx.Err())
				return
			default:
				chunk, err := srs[1].Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break outer
					}
				}
				fullMsgs = append(fullMsgs, chunk)
			}
		}
	}()

	return srs[0], nil
}

// GenerateWithTools 使用指定模型进行工具调用（支持 Function Calling）
func (x *Chat) GenerateWithTools(ctx context.Context, modelID string, messages []*schema.Message, tools []*schema.ToolInfo) (*schema.Message, error) {
	// 获取模型和参数
	chatModel, params, err := x.getModelAndParams(ctx, modelID)
	if err != nil {
		return nil, err
	}

	// 准备模型选项
	opts := params.ToModelOptions()

	// 如果有工具，添加工具选项
	if len(tools) > 0 {
		opts = append(opts, model.WithTools(tools))
		opts = append(opts, model.WithToolChoice(schema.ToolChoiceAllowed))
	}

	// 记录开始时间
	start := time.Now()

	result, err := chatModel.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("llm generate failed: %v", err)
	}

	// 计算延迟
	latencyMs := time.Since(start).Milliseconds()

	// 获取token使用量
	tokensUsed := 0
	if result.ResponseMeta != nil && result.ResponseMeta.Usage != nil {
		tokensUsed = result.ResponseMeta.Usage.TotalTokens
	}

	// 添加指标信息到返回的消息中（通过扩展字段）
	result.Extra = map[string]any{
		"latency_ms":  latencyMs,
		"tokens_used": tokensUsed,
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
