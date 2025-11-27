package kbgo

import (
	"context"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) DocumentsList(ctx context.Context, req *v1.DocumentsListReq) (res *v1.DocumentsListRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "DocumentsList request received - KnowledgeId: %s, Page: %d, Size: %d",
		req.KnowledgeId, req.Page, req.Size)

	documents, total, err := knowledge.GetDocumentsList(ctx, entity.KnowledgeDocuments{
		KnowledgeId: req.KnowledgeId,
	}, req.Page, req.Size)
	if err != nil {
		return
	}

	res = &v1.DocumentsListRes{
		Data:  documents,
		Total: total,
		Page:  req.Page,
		Size:  req.Size,
	}

	return
}
