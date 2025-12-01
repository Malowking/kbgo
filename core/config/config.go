package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/gogf/gf/v2/frame/g"
)

// ValidateConfiguration validates all required configuration items
func ValidateConfiguration(ctx context.Context) error {
	var missingConfigs []string
	var warnings []string

	// 检查向量数据库类型配置
	vectorStoreType := g.Cfg().MustGet(ctx, "vectorStore.type", "milvus").String()

	switch vectorStoreType {
	case "milvus":
		// 验证 Milvus 配置
		milvusAddress := g.Cfg().MustGet(ctx, "milvus.address", "").String()
		if milvusAddress == "" {
			missingConfigs = append(missingConfigs, "milvus.address")
		}
	case "postgresql":
		// 验证 PostgreSQL 配置
		pgHost := g.Cfg().MustGet(ctx, "postgres.host", "").String()
		pgUser := g.Cfg().MustGet(ctx, "postgres.user", "").String()
		pgDatabase := g.Cfg().MustGet(ctx, "postgres.database", "").String()

		if pgHost == "" {
			missingConfigs = append(missingConfigs, "postgres.host")
		}
		if pgUser == "" {
			missingConfigs = append(missingConfigs, "postgres.user")
		}
		if pgDatabase == "" {
			missingConfigs = append(missingConfigs, "postgres.database")
		}
	default:
		warnings = append(warnings, fmt.Sprintf("Unknown vector store type: %s, defaulting to milvus", vectorStoreType))
	}

	// 验证数据库配置
	dbHost := g.Cfg().MustGet(ctx, "database.default.host", "").String()
	dbPort := g.Cfg().MustGet(ctx, "database.default.port", "").String()
	dbUser := g.Cfg().MustGet(ctx, "database.default.user", "").String()
	dbName := g.Cfg().MustGet(ctx, "database.default.name", "").String()

	if dbHost == "" {
		missingConfigs = append(missingConfigs, "database.default.host")
	}
	if dbPort == "" {
		missingConfigs = append(missingConfigs, "database.default.port")
	}
	if dbUser == "" {
		missingConfigs = append(missingConfigs, "database.default.user")
	}
	if dbName == "" {
		missingConfigs = append(missingConfigs, "database.default.name")
	}

	// 输出警告信息
	if len(warnings) > 0 {
		g.Log().Warningf(ctx, "Configuration warnings:\n- %s", strings.Join(warnings, "\n- "))
	}

	// 检查是否有缺失的必需配置
	if len(missingConfigs) > 0 {
		return fmt.Errorf("missing required configuration items:\n- %s\n\nPlease check your config.yaml file and ensure all required settings are properly configured", strings.Join(missingConfigs, "\n- "))
	}

	// 输出成功信息
	g.Log().Info(ctx, "✓ All required configuration items are present")

	return nil
}

type Config struct {
	VectorStore vector_store.VectorStore // 向量数据库接口
	Database    string
	// embedding 时使用
	APIKey         string
	BaseURL        string
	EmbeddingModel string
	ChatModel      string
	// Milvus retriever 配置
	MetricType string // 向量相似度度量类型，如 "COSINE", "L2", "IP" 等，默认 "COSINE"
}

// RetrieverConfig Retriever专用配置，组合基础配置和向量存储
type RetrieverConfig struct {
	RetrieverConfigBase
	VectorStore vector_store.VectorStore // 向量数据库接口
}

// IndexerConfig Indexer专用配置
type IndexerConfig struct {
	VectorStore    vector_store.VectorStore // 向量数据库接口
	Database       string                   // 数据库名称
	APIKey         string                   // API密钥（用于调用embedding服务）
	BaseURL        string                   // API基础URL（用于调用embedding服务）
	EmbeddingModel string                   // Embedding模型名称
	MetricType     string                   // 向量相似度度量类型
}

// Config 实现 embedding config 接口
func (c *Config) GetAPIKey() string         { return c.APIKey }
func (c *Config) GetBaseURL() string        { return c.BaseURL }
func (c *Config) GetEmbeddingModel() string { return c.EmbeddingModel }

// RetrieverConfig 实现 embedding config 接口
func (c *RetrieverConfig) GetAPIKey() string         { return c.APIKey }
func (c *RetrieverConfig) GetBaseURL() string        { return c.BaseURL }
func (c *RetrieverConfig) GetEmbeddingModel() string { return c.EmbeddingModel }

// RetrieverConfig 实现 rerank config 接口
func (c *RetrieverConfig) GetRerankAPIKey() string  { return c.RerankAPIKey }
func (c *RetrieverConfig) GetRerankBaseURL() string { return c.RerankBaseURL }
func (c *RetrieverConfig) GetRerankModel() string   { return c.RerankModel }

// RetrieverConfig 实现 GeneralRetrieverConfig 接口
func (c *RetrieverConfig) GetTopK() int            { return c.TopK }
func (c *RetrieverConfig) GetScore() float64       { return c.Score }
func (c *RetrieverConfig) GetEnableRewrite() bool  { return c.EnableRewrite }
func (c *RetrieverConfig) GetRewriteAttempts() int { return c.RewriteAttempts }
func (c *RetrieverConfig) GetRetrieveMode() string { return c.RetrieveMode }

// IndexerConfig 实现 embedding config 接口
func (c *IndexerConfig) GetAPIKey() string         { return c.APIKey }
func (c *IndexerConfig) GetBaseURL() string        { return c.BaseURL }
func (c *IndexerConfig) GetEmbeddingModel() string { return c.EmbeddingModel }

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
		VectorStore: x.VectorStore,
		Database:    x.Database,
		// embedding 时使用
		APIKey:         x.APIKey,
		BaseURL:        x.BaseURL,
		EmbeddingModel: x.EmbeddingModel,
		ChatModel:      x.ChatModel,
		MetricType:     x.MetricType,
	}
}
