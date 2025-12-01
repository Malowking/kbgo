package config

// RetrieverConfigBase Retriever配置基础类型
type RetrieverConfigBase struct {
	MetricType     string // 向量相似度度量类型，如 "COSINE", "L2", "IP" 等，默认 "COSINE"
	APIKey         string // API密钥（用于调用embedding服务）
	BaseURL        string // API基础URL（用于调用embedding服务）
	EmbeddingModel string // Embedding模型名称
	// Rerank配置
	RerankAPIKey    string  // Rerank API密钥
	RerankBaseURL   string  // Rerank API基础URL
	RerankModel     string  // Rerank模型名称
	EnableRewrite   bool    // 是否启用查询重写（默认 false）
	RewriteAttempts int     // 查询重写尝试次数（默认 3）
	RetrieveMode    string  // 检索模式: milvus/rerank/rrf（默认 rerank）
	TopK            int     // 默认返回结果数量（默认 5）
	Score           float64 // 默认分数阈值（默认 0.2）
}

// RetrieverConfigBase 实现 embedding config 接口
func (c *RetrieverConfigBase) GetAPIKey() string         { return c.APIKey }
func (c *RetrieverConfigBase) GetBaseURL() string        { return c.BaseURL }
func (c *RetrieverConfigBase) GetEmbeddingModel() string { return c.EmbeddingModel }

// RetrieverConfigBase 实现 rerank config 接口
func (c *RetrieverConfigBase) GetRerankAPIKey() string  { return c.RerankAPIKey }
func (c *RetrieverConfigBase) GetRerankBaseURL() string { return c.RerankBaseURL }
func (c *RetrieverConfigBase) GetRerankModel() string   { return c.RerankModel }

// RetrieverConfigBase 实现 GeneralRetrieverConfig 接口
func (c *RetrieverConfigBase) GetTopK() int            { return c.TopK }
func (c *RetrieverConfigBase) GetScore() float64       { return c.Score }
func (c *RetrieverConfigBase) GetEnableRewrite() bool  { return c.EnableRewrite }
func (c *RetrieverConfigBase) GetRewriteAttempts() int { return c.RewriteAttempts }
func (c *RetrieverConfigBase) GetRetrieveMode() string { return c.RetrieveMode }
