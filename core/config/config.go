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
	}
}
