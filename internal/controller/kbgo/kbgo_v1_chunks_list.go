package kbgo

import (
	"context"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
)

func (c *ControllerV1) ChunksList(ctx context.Context, req *v1.ChunksListReq) (res *v1.ChunksListRes, err error) {
	chunks, total, err := knowledge.GetChunksList(ctx, entity.KnowledgeChunks{
		KnowledgeDocId: req.KnowledgeDocId,
	}, req.Page, req.Size)
	if err != nil {
		return
	}
	return &v1.ChunksListRes{
		Data:  chunks,
		Total: total,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}
