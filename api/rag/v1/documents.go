package v1

import (
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/gogf/gf/v2/frame/g"
)

const (
	StatusPending  Status = 0
	StatusIndexing Status = 1
	StatusActive   Status = 2
	StatusFailed   Status = 3
)

type DocumentsListReq struct {
	g.Meta      `path:"/v1/documents" method:"get" tags:"rag"`
	KnowledgeId string `p:"knowledge_id" dc:"knowledge_id" v:"required"`
	Page        int    `p:"page" dc:"page" v:"required|min:1" d:"1"`
	Size        int    `p:"size" dc:"size" v:"required|min:1|max:100" d:"10"`
}

type DocumentsListRes struct {
	g.Meta `mime:"application/json"`
	Data   []entity.KnowledgeDocuments `json:"data"`
	Total  int                         `json:"total"`
	Page   int                         `json:"page"`
	Size   int                         `json:"size"`
}

type DocumentsDeleteReq struct {
	g.Meta     `path:"/v1/documents" method:"delete" tags:"rag" summary:"Delete a document and its chunks"`
	DocumentId string `p:"document_id" dc:"document_id" v:"required"`
}

type DocumentsDeleteRes struct {
	g.Meta `mime:"application/json"`
}

type DocumentsReIndexReq struct {
	g.Meta      `path:"/v1/documents/reindex" method:"post" tags:"rag" summary:"Re-index a failed document"`
	DocumentId  string `p:"document_id" dc:"document_id" v:"required"`
	ChunkSize   int    `p:"chunk_size" dc:"chunk_size" d:"1000"`
	OverlapSize int    `p:"overlap_size" dc:"overlap_size" d:"100"`
}

type DocumentsReIndexRes struct {
	g.Meta   `mime:"application/json"`
	ChunkIds []string `json:"chunk_ids" dc:"chunk ids"`
}
