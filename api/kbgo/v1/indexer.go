package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// UploadFileReq 文件上传请求（原Indexer接口修改为纯上传）
type UploadFileReq struct {
	g.Meta      `path:"/v1/upload" method:"post" mime:"multipart/form-data" tags:"rag"`
	File        *ghttp.UploadFile `p:"file" type:"file" dc:"如果是本地文件，则直接上传文件"`
	URL         string            `p:"url" dc:"如果是网络文件则直接输入url即可"`
	KnowledgeId string            `p:"knowledge_id" dc:"知识库ID" v:"required"`
}

type UploadFileRes struct {
	g.Meta     `mime:"application/json"`
	DocumentId string `json:"document_id" dc:"文档ID"`
	Status     string `json:"status" dc:"上传状态"`
	Message    string `json:"message" dc:"状态消息"`
}

// IndexDocumentsReq 文件索引请求（批量切分并向量化）
type IndexDocumentsReq struct {
	g.Meta      `path:"/v1/index" method:"post" tags:"rag"`
	DocumentIds []string `p:"document_ids" dc:"文档ID列表" v:"required"`
	ChunkSize   int      `p:"chunk_size" dc:"文档分块大小" d:"1000"`
	OverlapSize int      `p:"overlap_size" dc:"分块重叠大小" d:"100"`
	Separator   string   `p:"separator" dc:"自定义分隔符，用于文档切分"`
}

type IndexDocumentsRes struct {
	g.Meta  `mime:"application/json"`
	Message string `json:"message" dc:"索引任务已启动"`
}
