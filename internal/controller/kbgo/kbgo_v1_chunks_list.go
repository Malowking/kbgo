package kbgo

import (
	"context"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) ChunksList(ctx context.Context, req *v1.ChunksListReq) (res *v1.ChunksListRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "ChunksList request received - KnowledgeDocId: %s, Page: %d, Size: %d",
		req.KnowledgeDocId, req.Page, req.Size)

	chunks, total, err := knowledge.GetChunksList(ctx, entity.KnowledgeChunks{
		KnowledgeDocId: req.KnowledgeDocId,
	}, req.Page, req.Size)
	if err != nil {
		return
	}

	// 转换 chunks 中的图片URL为可访问的代理URL
	r := g.RequestFromCtx(ctx)
	if r != nil {
		baseURL := common.GetBaseURL(r.Host, r.URL.Scheme, map[string]string{
			"X-Forwarded-Host":  r.Header.Get("X-Forwarded-Host"),
			"X-Forwarded-Proto": r.Header.Get("X-Forwarded-Proto"),
		})

		// 遍历所有 chunks 并转换图片URL
		for i := range chunks {
			// 将 entity.KnowledgeChunks 转换为 schema.Document 进行处理
			doc := &schema.Document{
				Content: chunks[i].Content,
			}
			common.ConvertImageURLsInDocument(doc, baseURL)
			chunks[i].Content = doc.Content
		}
	}

	return &v1.ChunksListRes{
		Data:  chunks,
		Total: total,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}
