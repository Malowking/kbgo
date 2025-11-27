package v1

import (
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

type RetrieverReq struct {
	g.Meta          `path:"/v1/retriever" method:"post" tags:"retriever"`
	Question        string  `json:"question" v:"required"`
	TopK            int     `json:"top_k"` // Default is 5
	Score           float64 `json:"score"` // Default is 0.2
	KnowledgeId     string  `json:"knowledge_id" v:"required"`
	EnableRewrite   bool    `json:"enable_rewrite"`   // Whether to enable query rewriting (default false)
	RewriteAttempts int     `json:"rewrite_attempts"` // Number of query rewriting attempts (default 3, only effective when enable_rewrite=true)
	RetrieveMode    string  `json:"retrieve_mode"`    // Retrieval mode: milvus/rerank/rrf (default rerank)
}

type RetrieverRes struct {
	g.Meta   `mime:"application/json"`
	Document []*schema.Document `json:"document"`
}
