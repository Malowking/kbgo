package client

import (
	"context"
	"fmt"

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
		Tools:               req.Tools,
		ToolChoice:          req.ToolChoice,
		ResponseFormat:      req.ResponseFormat,
	}

	resp, err := c.client.CreateChatCompletion(ctx, openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

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
