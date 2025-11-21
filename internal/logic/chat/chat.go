package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Malowking/kbgo/internal/history"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

var chatInstance *Chat

type Chat struct {
	cm     model.BaseChatModel
	eh     *history.Manager
	params ModelParams // 添加推理参数配置
}

func GetChat() *Chat {
	return chatInstance
}

func init() {
	ctx := gctx.New()

	// 加载聊天配置
	var chatCfg ChatConfig
	err := g.Cfg().MustGet(ctx, "chat").Scan(&chatCfg)
	if err != nil {
		g.Log().Fatalf(ctx, "load chat config failed, err=%v", err)
		return
	}

	c, err := newChat(&openai.ChatModelConfig{
		APIKey:  chatCfg.APIKey,
		BaseURL: chatCfg.BaseURL,
		Model:   chatCfg.Model,
	}, chatCfg.ModelParams)

	if err != nil {
		g.Log().Fatalf(ctx, "newChat failed, err=%v", err)
		return
	}
	c.eh = history.NewManager()
	chatInstance = c
}

func newChat(cfg *openai.ChatModelConfig, params ModelParams) (res *Chat, err error) {
	chatModel, err := openai.NewChatModel(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	// 如果没有设置参数，使用默认值
	if params.Temperature == nil && params.TopP == nil && params.MaxTokens == nil {
		params = GetDefaultParams()
	}

	return &Chat{
		cm:     chatModel,
		params: params,
	}, nil
}

func (x *Chat) GetAnswer(ctx context.Context, convID string, docs []*schema.Document, question string) (answer string, err error) {
	messages, err := x.docsMessages(ctx, convID, docs, question)
	if err != nil {
		return "", err
	}

	// 记录开始时间
	start := time.Now()

	result, err := x.generate(ctx, messages)
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

// GetAnswerStream 流式生成答案
func (x *Chat) GetAnswerStream(ctx context.Context, convID string, docs []*schema.Document, question string) (answer *schema.StreamReader[*schema.Message], err error) {
	messages, err := x.docsMessages(ctx, convID, docs, question)
	if err != nil {
		return
	}

	// 记录开始时间
	start := time.Now()

	ctx = context.Background()
	streamData, err := x.stream(ctx, messages)
	if err != nil {
		err = fmt.Errorf("生成答案失败: %w", err)
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

func (x *Chat) generate(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	opts := x.params.ToModelOptions()
	result, err := x.cm.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("llm generate failed: %v", err)
	}
	return result, nil
}

func (x *Chat) stream(ctx context.Context, messages []*schema.Message) (*schema.StreamReader[*schema.Message], error) {
	opts := x.params.ToModelOptions()
	result, err := x.cm.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("llm stream failed: %v", err)
	}
	return result, nil
}

func (x *Chat) Generate(ctx context.Context, convID string, docs []*schema.Document, question string, customParams *ModelParams) (string, error) {
	messages, err := x.docsMessages(ctx, convID, docs, question)
	if err != nil {
		return "", err
	}

	// 使用自定义参数或默认参数
	params := x.params
	if customParams != nil {
		params = *customParams
	}

	// 记录开始时间
	start := time.Now()

	opts := params.ToModelOptions()
	result, err := x.cm.Generate(ctx, messages, opts...)
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
		return "", err
	}

	return result.Content, nil
}

// UpdateParams 更新模型参数
func (x *Chat) UpdateParams(params ModelParams) {
	x.params = params
}

// GetParams 获取当前模型参数
func (x *Chat) GetParams() ModelParams {
	return x.params
}

// GenerateWithTools 使用工具进行生成（支持 Function Calling）
func (x *Chat) GenerateWithTools(ctx context.Context, messages []*schema.Message, tools []*schema.ToolInfo) (*schema.Message, error) {
	// 准备模型选项
	opts := x.params.ToModelOptions()

	// 如果有工具，添加工具选项
	if len(tools) > 0 {
		opts = append(opts, model.WithTools(tools))
		opts = append(opts, model.WithToolChoice(schema.ToolChoiceAllowed))
	}

	// 记录开始时间
	start := time.Now()

	result, err := x.cm.Generate(ctx, messages, opts...)
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
	// 注意：LatencyMs 和 TokensUsed 不再是 Message 的直接字段，需要通过其他方式处理
	result.Extra = map[string]any{
		"latency_ms":  latencyMs,
		"tokens_used": tokensUsed,
	}

	return result, nil
}

// GenerateWithToolsAndSave 使用工具进行生成并返回结果（不自动保存）
func (x *Chat) GenerateWithToolsAndSave(ctx context.Context, messages []*schema.Message, tools []*schema.ToolInfo) (*schema.Message, error) {
	// 准备模型选项
	opts := x.params.ToModelOptions()

	// 如果有工具，添加工具选项
	if len(tools) > 0 {
		opts = append(opts, model.WithTools(tools))
		opts = append(opts, model.WithToolChoice(schema.ToolChoiceAllowed))
	}

	result, err := x.cm.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("llm generate failed: %v", err)
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

// ConcatStreamContent 将流式内容连接成完整字符串
func (x *Chat) ConcatStreamContent(messages []*schema.Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		builder.WriteString(msg.Content)
	}
	return builder.String()
}
