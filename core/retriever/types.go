package retriever

// RetrieveMode 定义检索模式
type RetrieveMode string

const (
	// RetrieveModeMilvus 仅使用Milvus向量检索，按相似度排序
	RetrieveModeMilvus RetrieveMode = "milvus"
	// RetrieveModeRerank 使用Milvus检索后进行Rerank重排序（默认）
	RetrieveModeRerank RetrieveMode = "rerank"
	// RetrieveModeRRF 使用RRF (Reciprocal Rank Fusion) 混合检索
	RetrieveModeRRF RetrieveMode = "rrf"
)

// RetrieveReq 检索请求参数
// Query 和 KnowledgeId 是必需的
// 其他参数是可选的，如果不提供则使用 RetrieverConfig 中的默认值
type RetrieveReq struct {
	Query       string // 检索关键词（必需）
	KnowledgeId string // 知识库ID（必需）

	// 以下参数可选，使用指针类型表示可选
	// 如果为 nil，则使用 RetrieverConfig 中的默认值
	TopK            *int          // 检索结果数量（可选）
	Score           *float64      // 分数阀值（可选，0-1范围）
	EnableRewrite   *bool         // 是否启用查询重写（可选）
	RewriteAttempts *int          // 查询重写尝试次数（可选）
	RetrieveMode    *RetrieveMode // 检索模式（可选）

	// 内部使用字段
	optQuery   string   // 优化后的检索关键词（内部使用）
	excludeIDs []string // 要排除的 _id 列表（内部使用）
}

// Copy 创建请求的副本
func (r *RetrieveReq) Copy() *RetrieveReq {
	return &RetrieveReq{
		Query:           r.Query,
		KnowledgeId:     r.KnowledgeId,
		TopK:            r.TopK,
		Score:           r.Score,
		EnableRewrite:   r.EnableRewrite,
		RewriteAttempts: r.RewriteAttempts,
		RetrieveMode:    r.RetrieveMode,
		optQuery:        r.optQuery,
		excludeIDs:      r.excludeIDs,
	}
}
