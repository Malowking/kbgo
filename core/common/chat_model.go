package common

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Malowking/kbgo/core/model"
	modelRegistry "github.com/Malowking/kbgo/core/model"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/gogf/gf/v2/frame/g"
)

var (
	embeddingModel einoModel.BaseChatModel
	rerankModel    einoModel.BaseChatModel
	rewriteModel   einoModel.BaseChatModel
	chatModel      einoModel.BaseChatModel
)

func GetChatModel(ctx context.Context, cfg *openai.ChatModelConfig) (einoModel.BaseChatModel, error) {
	if chatModel != nil {
		return chatModel, nil
	}
	if cfg == nil {
		cfg = &openai.ChatModelConfig{}
		err := g.Cfg().MustGet(ctx, "chat").Scan(cfg)
		if err != nil {
			return nil, err
		}
	}
	cm, err := openai.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	chatModel = cm
	return cm, nil
}

func GetEmbeddingModel(ctx context.Context, cfg *openai.ChatModelConfig) (einoModel.BaseChatModel, error) {
	if embeddingModel != nil {
		return embeddingModel, nil
	}
	if cfg == nil {
		cfg = &openai.ChatModelConfig{}
		err := g.Cfg().MustGet(ctx, "embedding").Scan(cfg)
		if err != nil {
			return nil, err
		}
	}
	cm, err := openai.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	embeddingModel = cm
	return cm, nil
}

func GetRewriteModel(ctx context.Context, cfg *qwen.ChatModelConfig) (einoModel.BaseChatModel, error) {
	// 每次都从注册表中重新获取，确保使用的是最新的模型配置
	// 从注册表中获取所有 LLM 类型的模型
	llmModels := modelRegistry.Registry.GetByType(model.ModelTypeLLM)

	// 如果没有注册任何 LLM 模型，返回错误
	if len(llmModels) == 0 {
		return nil, fmt.Errorf("no LLM models registered in registry")
	}

	// 随机选择一个 LLM 模型
	rand.Seed(time.Now().UnixNano())
	selectedModel := llmModels[rand.Intn(len(llmModels))]

	g.Log().Infof(ctx, "Randomly selected LLM model for rewrite: %s (ID: %s, Provider: %s)",
		selectedModel.Name, selectedModel.ModelID, selectedModel.Provider)

	// 使用选中的模型创建 ChatModel
	modelCfg := &openai.ChatModelConfig{
		BaseURL: selectedModel.BaseURL,
		APIKey:  selectedModel.APIKey,
		Model:   selectedModel.Name,
	}

	cm, err := openai.NewChatModel(ctx, modelCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model with selected LLM: %w", err)
	}

	return cm, nil
}

func GetRerankModel(ctx context.Context, cfg *openai.ChatModelConfig) (einoModel.BaseChatModel, error) {
	if rerankModel != nil {
		return rerankModel, nil
	}
	if cfg == nil {
		cfg = &openai.ChatModelConfig{}
		err := g.Cfg().MustGet(ctx, "rerank").Scan(cfg)
		if err != nil {
			return nil, err
		}
	}
	cm, err := openai.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	rerankModel = cm
	return cm, nil
}
