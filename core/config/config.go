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

	// 验证 Milvus 配置
	milvusAddress := g.Cfg().MustGet(ctx, "milvus.address", "").String()
	if milvusAddress == "" {
		missingConfigs = append(missingConfigs, "milvus.address")
	}

	// 验证 Embedding 配置
	embeddingAPIKey := g.Cfg().MustGet(ctx, "embedding.apiKey", "").String()
	embeddingBaseURL := g.Cfg().MustGet(ctx, "embedding.baseURL", "").String()
	embeddingModel := g.Cfg().MustGet(ctx, "embedding.model", "").String()

	if embeddingAPIKey == "" {
		missingConfigs = append(missingConfigs, "embedding.apiKey")
	}
	if embeddingBaseURL == "" {
		missingConfigs = append(missingConfigs, "embedding.baseURL")
	}
	if embeddingModel == "" {
		missingConfigs = append(missingConfigs, "embedding.model")
	}

	// 验证 Chat 配置
	chatAPIKey := g.Cfg().MustGet(ctx, "chat.apiKey", "").String()
	chatBaseURL := g.Cfg().MustGet(ctx, "chat.baseURL", "").String()
	chatModel := g.Cfg().MustGet(ctx, "chat.model", "").String()

	if chatAPIKey == "" {
		warnings = append(warnings, "chat.apiKey is not set")
	}
	if chatBaseURL == "" {
		warnings = append(warnings, "chat.baseURL is not set")
	}
	if chatModel == "" {
		warnings = append(warnings, "chat.model is not set")
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

// RetrieverConfig Retriever专用配置
type RetrieverConfig struct {
	VectorStore    vector_store.VectorStore // 向量数据库接口
	MetricType     string                   // 向量相似度度量类型，如 "COSINE", "L2", "IP" 等，默认 "COSINE"
	APIKey         string                   // API密钥（用于调用embedding服务）
	BaseURL        string                   // API基础URL（用于调用embedding服务）
	EmbeddingModel string                   // Embedding模型名称
	// 检索策略参数
	// TODO 重写的话需要一个大模型
	EnableRewrite   bool    // 是否启用查询重写（默认 false）
	RewriteAttempts int     // 查询重写尝试次数（默认 3）
	RetrieveMode    string  // 检索模式: milvus/rerank/rrf（默认 rerank）
	TopK            int     // 默认返回结果数量（默认 5）
	Score           float64 // 默认分数阈值（默认 0.2）
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
