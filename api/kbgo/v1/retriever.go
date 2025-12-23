package v1

import (
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

type RetrieverReq struct {
	g.Meta           `path:"/v1/retriever" method:"post" tags:"retriever"`
	Question         string   `json:"question" v:"required"`
	EmbeddingModelID string   `json:"embedding_model_id"` // Embedding模型UUID（可选，如果不提供则使用知识库绑定的模型）
	RerankModelID    string   `json:"rerank_model_id"`    // Rerank模型UUID（可选，仅在retrieve_mode为rerank或rrf时需要）
	TopK             int      `json:"top_k"`              // Default is 5
	Score            float64  `json:"score"`              // Default is 0.2
	KnowledgeId      string   `json:"knowledge_id" v:"required"`
	EnableRewrite    bool     `json:"enable_rewrite"`   // Whether to enable query rewriting (default false)
	RewriteAttempts  int      `json:"rewrite_attempts"` // Number of query rewriting attempts (default 3, only effective when enable_rewrite=true)
	RetrieveMode     string   `json:"retrieve_mode"`    // Retrieval mode: milvus/rerank/rrf (default rerank)
	RerankWeight     *float64 `json:"rerank_weight"`    // Rerank权重 (0-1范围，默认1.0)，1.0为纯rerank，0.0为纯BM25，中间值为混合
}

type RetrieverRes struct {
	g.Meta   `mime:"application/json"`
	Document []*schema.Document `json:"document"`
}
