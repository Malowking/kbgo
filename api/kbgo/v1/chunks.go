package v1

import (
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/gogf/gf/v2/frame/g"
)

type ChunksListReq struct {
	g.Meta         `path:"/v1/chunks" method:"get" tags:"retriever"`
	KnowledgeDocId string `p:"knowledge_doc_id" dc:"knowledge_doc_id" v:"required"`
	Page           int    `p:"page" dc:"page" v:"required|min:1" d:"1"`
	Size           int    `p:"size" dc:"size" v:"required|min:1|max:100" d:"10"`
}

type ChunksListRes struct {
	g.Meta `mime:"application/json"`
	Data   []entity.KnowledgeChunks `json:"data"`
	Total  int                      `json:"total"`
	Page   int                      `json:"page"`
	Size   int                      `json:"size"`
}

type ChunkDeleteReq struct {
	g.Meta  `path:"/v1/chunks" method:"delete" tags:"retriever"`
	ChunkId string `p:"id" dc:"id" v:"required"`
}

type ChunkDeleteRes struct {
	g.Meta `mime:"application/json"`
}

type UpdateChunkReq struct {
	g.Meta `path:"/v1/chunks" method:"put" tags:"retriever"`
	Ids    []string `p:"ids" dc:"ids" v:"required"`
	Status int      `p:"status" dc:"status" v:"required|in:0,1"`
}

type UpdateChunkRes struct {
	g.Meta `mime:"application/json"`
}
