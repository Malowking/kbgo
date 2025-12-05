package common

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Malowking/kbgo/core/client"
	"github.com/Malowking/kbgo/core/model"
	modelRegistry "github.com/Malowking/kbgo/core/model"
	"github.com/gogf/gf/v2/frame/g"
)

// ChatModelConfig OpenAI 聊天模型配置
type ChatModelConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

var (
	chatClient      *client.OpenAIClient
	embeddingClient *client.OpenAIClient
	rerankClient    *client.OpenAIClient
)

// GetChatClient 获取聊天客户端
func GetChatClient(ctx context.Context, cfg *ChatModelConfig) (*client.OpenAIClient, error) {
	if chatClient != nil {
		return chatClient, nil
	}
	if cfg == nil {
		cfg = &ChatModelConfig{}
		err := g.Cfg().MustGet(ctx, "chat").Scan(cfg)
		if err != nil {
			return nil, err
		}
	}
	chatClient = client.NewOpenAIClient(cfg.APIKey, cfg.BaseURL)
	return chatClient, nil
}

// GetEmbeddingClient 获取 embedding 客户端
func GetEmbeddingClient(ctx context.Context, cfg *ChatModelConfig) (*client.OpenAIClient, error) {
	if embeddingClient != nil {
		return embeddingClient, nil
	}
	if cfg == nil {
		cfg = &ChatModelConfig{}
		err := g.Cfg().MustGet(ctx, "embedding").Scan(cfg)
		if err != nil {
			return nil, err
		}
	}
	embeddingClient = client.NewOpenAIClient(cfg.APIKey, cfg.BaseURL)
	return embeddingClient, nil
}

// GetRewriteClient 获取重写客户端（从模型注册表中随机选择）
func GetRewriteClient(ctx context.Context) (*client.OpenAIClient, error) {
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

	// 使用选中的模型创建 OpenAI 客户端
	return client.NewOpenAIClient(selectedModel.APIKey, selectedModel.BaseURL), nil
}

// GetRerankClient 获取 rerank 客户端
func GetRerankClient(ctx context.Context, cfg *ChatModelConfig) (*client.OpenAIClient, error) {
	if rerankClient != nil {
		return rerankClient, nil
	}
	if cfg == nil {
		cfg = &ChatModelConfig{}
		err := g.Cfg().MustGet(ctx, "rerank").Scan(cfg)
		if err != nil {
			return nil, err
		}
	}
	rerankClient = client.NewOpenAIClient(cfg.APIKey, cfg.BaseURL)
	return rerankClient, nil
}
