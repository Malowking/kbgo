package rag

import (
	"context"

	v1 "github.com/Malowking/kbgo/api/rag/v1"
	"github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) DocumentsReIndex(ctx context.Context, req *v1.DocumentsReIndexReq) (res *v1.DocumentsReIndexRes, err error) {
	svr := rag.GetRagSvr()

	// 调用ReIndex方法
	reindexReq := &core.ReIndexReq{
		DocumentId:  req.DocumentId,
		ChunkSize:   req.ChunkSize,
		OverlapSize: req.OverlapSize,
	}

	ids, err := svr.ReIndex(ctx, reindexReq)
	if err != nil {
		g.Log().Errorf(ctx, "DocumentsReIndex failed, document_id=%s, err=%v", req.DocumentId, err)
		return nil, err
	}

	g.Log().Infof(ctx, "DocumentsReIndex success, document_id=%s, chunk_count=%d", req.DocumentId, len(ids))

	return &v1.DocumentsReIndexRes{
		ChunkIds: ids,
	}, nil
}
