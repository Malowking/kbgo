package v1

import (
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

type RetrieverReq struct {
	g.Meta          `path:"/v1/retriever" method:"post" tags:"rag"`
	Question        string  `json:"question" v:"required"`
	TopK            int     `json:"top_k"` // 默认为5
	Score           float64 `json:"score"` // 默认为0.2
	KnowledgeId     string  `json:"knowledge_id" v:"required"`
	EnableRewrite   bool    `json:"enable_rewrite"`   // 是否启用查询重写（默认 false）
	RewriteAttempts int     `json:"rewrite_attempts"` // 查询重写尝试次数（默认 3，仅在 enable_rewrite=true 时生效）
	RetrieveMode    string  `json:"retrieve_mode"`    // 检索模式: milvus/rerank/rrf（默认 rerank）
}

type RetrieverRes struct {
	g.Meta   `mime:"application/json"`
	Document []*schema.Document `json:"document"`
}
