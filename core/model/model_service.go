package model

import (
	"context"

	"github.com/Malowking/kbgo/core/client"
	"github.com/Malowking/kbgo/core/errors"
	formatterPkg "github.com/Malowking/kbgo/core/formatter"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/sashabaranov/go-openai"
)

// ModelService 统一的模型服务
type ModelService struct {
	client    *client.OpenAIClient
	formatter formatterPkg.MessageFormatter
}

// NewModelService 创建模型服务
func NewModelService(apiKey, baseURL string, formatter formatterPkg.MessageFormatter) *ModelService {
	// 如果formatter为nil，使用默认的OpenAI formatter
	if formatter == nil {
		formatter = formatterPkg.NewOpenAIFormatter()
	}
	return &ModelService{
		client:    client.NewOpenAIClient(apiKey, baseURL),
		formatter: formatter,
	}
}

// ChatCompletionParams 聊天参数
type ChatCompletionParams struct {
	ModelName           string
	Messages            []*schema.Message
	Temperature         float32
	MaxCompletionTokens int
	TopP                float32
	FrequencyPenalty    float32
	PresencePenalty     float32
	N                   int
	Stop                []string
	Tools               []openai.Tool
	ToolChoice          any
	ResponseFormat      *openai.ChatCompletionResponseFormat
}

// ChatCompletion 非流式对话
func (s *ModelService) ChatCompletion(ctx context.Context, params ChatCompletionParams) (*openai.ChatCompletionResponse, error) {
	// 使用格式适配器转换消息
	openaiMessages, err := s.formatter.FormatMessages(params.Messages)
	if err != nil {
		return nil, errors.Newf(errors.ErrInvalidParameter, "failed to format messages: %v", err)
	}

	// 调用客户端
	req := client.ChatCompletionRequest{
		Model:               params.ModelName,
		Messages:            openaiMessages,
		Temperature:         params.Temperature,
		MaxCompletionTokens: params.MaxCompletionTokens,
		TopP:                params.TopP,
		FrequencyPenalty:    params.FrequencyPenalty,
		PresencePenalty:     params.PresencePenalty,
		N:                   params.N,
		Stop:                params.Stop,
		Tools:               params.Tools,
		ToolChoice:          params.ToolChoice,
		ResponseFormat:      params.ResponseFormat,
	}

	return s.client.ChatCompletion(ctx, req)
}

// ChatCompletionStream 流式对话
func (s *ModelService) ChatCompletionStream(ctx context.Context, params ChatCompletionParams) (*openai.ChatCompletionStream, error) {
	// 使用格式适配器转换消息
	openaiMessages, err := s.formatter.FormatMessages(params.Messages)
	if err != nil {
		return nil, errors.Newf(errors.ErrInvalidParameter, "failed to format messages: %v", err)
	}

	// 调用客户端
	req := client.ChatCompletionRequest{
		Model:               params.ModelName,
		Messages:            openaiMessages,
		Temperature:         params.Temperature,
		MaxCompletionTokens: params.MaxCompletionTokens,
		TopP:                params.TopP,
		FrequencyPenalty:    params.FrequencyPenalty,
		PresencePenalty:     params.PresencePenalty,
		N:                   params.N,
		Stop:                params.Stop,
		Tools:               params.Tools,
		ToolChoice:          params.ToolChoice,
		ResponseFormat:      params.ResponseFormat,
		Stream:              true,
	}

	return s.client.ChatCompletionStream(ctx, req)
}
