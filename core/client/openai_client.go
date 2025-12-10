package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/sashabaranov/go-openai"
)

// OpenAIClient 统一的OpenAI API客户端
// 负责处理所有OpenAI格式的HTTP请求，包括流式和非流式调用
type OpenAIClient struct {
	client *openai.Client
}

// NewOpenAIClient 创建OpenAI客户端
func NewOpenAIClient(apiKey, baseURL string) *OpenAIClient {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	return &OpenAIClient{
		client: openai.NewClientWithConfig(config),
	}
}

// ChatCompletionRequest 聊天请求参数
type ChatCompletionRequest struct {
	Model               string
	Messages            []openai.ChatCompletionMessage
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
	Stream              bool
}

// ChatCompletion 非流式对话
func (c *OpenAIClient) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	openaiReq := openai.ChatCompletionRequest{
		Model:               req.Model,
		Messages:            req.Messages,
		Temperature:         req.Temperature,
		MaxCompletionTokens: req.MaxCompletionTokens,
		TopP:                req.TopP,
		FrequencyPenalty:    req.FrequencyPenalty,
		PresencePenalty:     req.PresencePenalty,
		N:                   req.N,
		Stop:                req.Stop,
		Tools:               req.Tools,
		ToolChoice:          req.ToolChoice,
		ResponseFormat:      req.ResponseFormat,
	}

	// 记录请求详情
	g.Log().Infof(ctx, "[OpenAI Client] 发送请求 - Model: %s, Messages: %d, Tools: %d, Temp: %.2f, MaxTokens: %d",
		req.Model, len(req.Messages), len(req.Tools), req.Temperature, req.MaxCompletionTokens)

	resp, err := c.client.CreateChatCompletion(ctx, openaiReq)
	if err != nil {
		// 添加调试信息
		g.Log().Errorf(ctx, "[OpenAI Client] API调用失败 - Model: %s, Error: %v", req.Model, err)
		if debugJSON, jsonErr := json.MarshalIndent(req.Messages, "", "  "); jsonErr == nil {
			g.Log().Debugf(ctx, "[OpenAI Client] 失败请求的消息:\n%s", string(debugJSON))
		}
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	// 记录响应详情
	g.Log().Infof(ctx, "[OpenAI Client] 收到响应 - ID: %s, Model: %s, Choices: %d, Usage: %+v",
		resp.ID, resp.Model, len(resp.Choices), resp.Usage)

	return &resp, nil
}

// ChatCompletionStream 流式对话
func (c *OpenAIClient) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	openaiReq := openai.ChatCompletionRequest{
		Model:               req.Model,
		Messages:            req.Messages,
		Temperature:         req.Temperature,
		MaxCompletionTokens: req.MaxCompletionTokens,
		TopP:                req.TopP,
		FrequencyPenalty:    req.FrequencyPenalty,
		PresencePenalty:     req.PresencePenalty,
		N:                   req.N,
		Stop:                req.Stop,
		Tools:               req.Tools,
		ToolChoice:          req.ToolChoice,
		ResponseFormat:      req.ResponseFormat,
		Stream:              true,
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion stream: %w", err)
	}

	return stream, nil
}
