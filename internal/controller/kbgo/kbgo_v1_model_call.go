package kbgo

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/model"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/sashabaranov/go-openai"
)

// ChatCompletion OpenAI 风格聊天接口
func (c *ControllerV1) ChatCompletion(ctx context.Context, req *v1.ChatCompletionReq) (res *v1.ChatCompletionRes, err error) {
	g.Log().Infof(ctx, "ChatCompletion request received - ModelID: %s, Messages: %d", req.ModelID, len(req.Messages))

	// 获取模型配置
	mc := model.Registry.Get(req.ModelID)
	if mc == nil {
		g.Log().Errorf(ctx, "Model not found: %s", req.ModelID)
		return nil, errors.Newf(errors.ErrModelNotFound, "model not found: %s", req.ModelID)
	}

	// 转换消息格式
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Name:    msg.Name,
		}

		// 转换工具调用
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]openai.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				toolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolType(tc.Type),
					Function: openai.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
			messages[i].ToolCalls = toolCalls
		}

		if msg.ToolCallID != "" {
			messages[i].ToolCallID = msg.ToolCallID
		}
	}

	// 从模型配置中获取默认参数
	defaultMaxCompletionTokens := 4096
	defaultTemperature := float32(0.7)
	defaultTopP := float32(0.9)
	defaultFrequencyPenalty := float32(0.0)
	defaultPresencePenalty := float32(0.0)
	var defaultStop []string

	// 解析模型配置中的默认参数
	if mc.Extra != nil {
		if maxCompletionTokens, ok := mc.Extra["maxCompletionTokens"].(float64); ok {
			defaultMaxCompletionTokens = int(maxCompletionTokens)
		}
		if temp, ok := mc.Extra["temperature"].(float64); ok {
			defaultTemperature = float32(temp)
		}
		if topP, ok := mc.Extra["topP"].(float64); ok {
			defaultTopP = float32(topP)
		}
		if freqPenalty, ok := mc.Extra["frequencyPenalty"].(float64); ok {
			defaultFrequencyPenalty = float32(freqPenalty)
		}
		if presPenalty, ok := mc.Extra["presencePenalty"].(float64); ok {
			defaultPresencePenalty = float32(presPenalty)
		}
		if stop, ok := mc.Extra["stop"].([]interface{}); ok {
			stopWords := make([]string, 0, len(stop))
			for _, s := range stop {
				if str, ok := s.(string); ok {
					stopWords = append(stopWords, str)
				}
			}
			defaultStop = stopWords
		}
	}

	// 使用请求参数覆盖默认值
	maxCompletionTokens := defaultMaxCompletionTokens
	if req.MaxTokens > 0 {
		maxCompletionTokens = req.MaxTokens
	}

	temperature := defaultTemperature
	if req.Temperature > 0 {
		temperature = req.Temperature
	}

	topP := defaultTopP
	if req.TopP > 0 {
		topP = req.TopP
	}

	frequencyPenalty := defaultFrequencyPenalty
	if req.FrequencyPenalty != 0 {
		frequencyPenalty = req.FrequencyPenalty
	}

	presencePenalty := defaultPresencePenalty
	if req.PresencePenalty != 0 {
		presencePenalty = req.PresencePenalty
	}

	stop := defaultStop
	if len(req.Stop) > 0 {
		stop = req.Stop
	}

	// 构建请求
	chatReq := openai.ChatCompletionRequest{
		Model:               mc.Name, // 使用模型名称而非UUID
		Messages:            messages,
		MaxCompletionTokens: maxCompletionTokens,
		Temperature:         temperature,
		TopP:                topP,
		FrequencyPenalty:    frequencyPenalty,
		PresencePenalty:     presencePenalty,
		Stop:                stop,
		Stream:              req.Stream,
	}

	// 转换工具定义
	if len(req.Tools) > 0 {
		tools := make([]openai.Tool, len(req.Tools))
		for i, tool := range req.Tools {
			tools[i] = openai.Tool{
				Type: openai.ToolType(tool.Type),
				Function: &openai.FunctionDefinition{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}
		}
		chatReq.Tools = tools

		if req.ToolChoice != nil {
			chatReq.ToolChoice = req.ToolChoice
			g.Log().Infof(ctx, "Tools enabled - Count: %d, ToolChoice: %v (user specified)", len(tools), req.ToolChoice)
		} else {
			chatReq.ToolChoice = "auto"
			g.Log().Infof(ctx, "Tools enabled - Count: %d, ToolChoice: auto (default)", len(tools))
		}
	}

	// 调用模型
	resp, err := mc.Client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to call model: %v", err)
		return nil, err
	}

	// 转换响应格式
	choices := make([]v1.ChatCompletionChoice, len(resp.Choices))
	for i, choice := range resp.Choices {
		msg := v1.ChatCompletionMessage{
			Role:    choice.Message.Role,
			Content: choice.Message.Content,
			Name:    choice.Message.Name,
		}

		// 转换工具调用
		if len(choice.Message.ToolCalls) > 0 {
			toolCalls := make([]v1.ChatCompletionToolCall, len(choice.Message.ToolCalls))
			for j, tc := range choice.Message.ToolCalls {
				toolCalls[j] = v1.ChatCompletionToolCall{
					ID:   tc.ID,
					Type: string(tc.Type),
					Function: v1.ChatCompletionToolCallFunc{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
			msg.ToolCalls = toolCalls
		}

		choices[i] = v1.ChatCompletionChoice{
			Index:        choice.Index,
			Message:      msg,
			FinishReason: string(choice.FinishReason),
		}
	}

	return &v1.ChatCompletionRes{
		ID:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
		Choices: choices,
		Usage: v1.ChatCompletionUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// EmbeddingCompletion 向量化接口
func (c *ControllerV1) EmbeddingCompletion(ctx context.Context, req *v1.EmbeddingReq) (res *v1.EmbeddingRes, err error) {
	g.Log().Infof(ctx, "EmbeddingCompletion request received - ModelID: %s, Input: %d items", req.ModelID, len(req.Input))

	// 获取模型配置
	mc := model.Registry.Get(req.ModelID)
	if mc == nil {
		g.Log().Errorf(ctx, "Model not found: %s", req.ModelID)
		return nil, errors.Newf(errors.ErrModelNotFound, "model not found: %s", req.ModelID)
	}

	// 确保是 embedding 模型
	if mc.Type != model.ModelTypeEmbedding {
		g.Log().Errorf(ctx, "Model %s is not an embedding model, type: %s", req.ModelID, mc.Type)
		return nil, errors.Newf(errors.ErrModelConfigInvalid, "model %s is not an embedding model, type: %s", req.ModelID, mc.Type)
	}

	// 调用 embedding 接口
	embReq := openai.EmbeddingRequest{
		Input: req.Input,
		Model: openai.EmbeddingModel(mc.Name),
	}

	resp, err := mc.Client.CreateEmbeddings(ctx, embReq)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to create embeddings: %v", err)
		return nil, err
	}

	// 转换响应
	data := make([]v1.EmbeddingData, len(resp.Data))
	for i, d := range resp.Data {
		data[i] = v1.EmbeddingData{
			Index:     d.Index,
			Embedding: d.Embedding,
		}
	}

	return &v1.EmbeddingRes{
		Object: resp.Object,
		Data:   data,
		Model:  string(resp.Model),
		Usage: v1.EmbeddingUsage{
			PromptTokens: resp.Usage.PromptTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}
