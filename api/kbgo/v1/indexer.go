package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// UploadFileReq File upload request (Indexer interface modified to pure upload)
type UploadFileReq struct {
	g.Meta      `path:"/v1/upload" method:"post" mime:"multipart/form-data" tags:"retriever"`
	File        *ghttp.UploadFile `p:"file" type:"file" dc:"If it's a local file, upload the file directly"`
	URL         string            `p:"url" dc:"If it's a web file, just enter the URL" d:""`
	KnowledgeId string            `p:"knowledge_id" dc:"Knowledge base ID" v:"required"`
}

type UploadFileRes struct {
	g.Meta     `mime:"application/json"`
	DocumentId string `json:"document_id" dc:"Document ID"`
	Status     string `json:"status" dc:"Upload status"`
	Message    string `json:"message" dc:"Status message"`
}

// IndexDocumentsReq Document indexing request (batch splitting and vectorization)
type IndexDocumentsReq struct {
	g.Meta           `path:"/v1/index" method:"post" tags:"retriever"`
	DocumentIds      []string `p:"document_ids" dc:"Document ID list" v:"required"`
	EmbeddingModelID string   `p:"embedding_model_id" dc:"Embedding model UUID" v:"required"` // Embedding模型UUID（必填）
	ChunkSize        int      `p:"chunk_size" dc:"Document chunk size" d:"1000"`
	OverlapSize      int      `p:"overlap_size" dc:"Chunk overlap size" d:"100"`
	Separator        string   `p:"separator" dc:"Custom separator for document splitting"`
}

type IndexDocumentsRes struct {
	g.Meta  `mime:"application/json"`
	Message string `json:"message" dc:"Indexing task started"`
}
