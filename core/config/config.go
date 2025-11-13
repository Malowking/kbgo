package config

import (
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

type Config struct {
	Client   *milvusclient.Client
	Database string
	// embedding 时使用
	APIKey         string
	BaseURL        string
	EmbeddingModel string
	ChatModel      string
	// Milvus retriever 配置
	MetricType string // 向量相似度度量类型，如 "COSINE", "L2", "IP" 等，默认 "COSINE"
}

func (x *Config) GetChatModelConfig() *openai.ChatModelConfig {
	if x == nil {
		return nil
	}
	return &openai.ChatModelConfig{
		APIKey:  x.APIKey,
		BaseURL: x.BaseURL,
		Model:   x.ChatModel,
	}
}

func (x *Config) Copy() *Config {
	return &Config{
		Client:   x.Client,
		Database: x.Database,
		// embedding 时使用
		APIKey:         x.APIKey,
		BaseURL:        x.BaseURL,
		EmbeddingModel: x.EmbeddingModel,
		ChatModel:      x.ChatModel,
		MetricType:     x.MetricType,
	}
}
