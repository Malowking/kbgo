package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

type IndexerReq struct {
	g.Meta      `path:"/v1/indexer" method:"post" mime:"multipart/form-data" tags:"rag"`
	File        *ghttp.UploadFile `p:"file" type:"file" dc:"如果是本地文件，则直接上传文件"`
	URL         string            `p:"url" dc:"如果是网络文件则直接输入url即可"`
	KnowledgeId string            `p:"knowledge_id" dc:"知识库ID" v:"required"`
	IsQA        int               `p:"is_qa" dc:"是否进行QA切分文档" d:"0"`
	ChunkSize   int               `p:"chunk_size" dc:"文档分块大小" d:"1000"`
	OverlapSize int               `p:"overlap_size" dc:"分块重叠大小" d:"100"`
}

type IndexerRes struct {
	g.Meta `mime:"application/json"`
	DocIDs []string `json:"doc_ids"`
}
