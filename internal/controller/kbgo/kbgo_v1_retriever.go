package kbgo

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/service"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) Retriever(ctx context.Context, req *v1.RetrieverReq) (res *v1.RetrieverRes, err error) {
	g.Log().Infof(ctx, "ragReq: %v, EnableRewrite: %v, RewriteAttempts: %v, RetrieveMode: %v", req, req.EnableRewrite, req.RewriteAttempts, req.RetrieveMode)

	// 使用共享的检索服务
	retrieverService := service.GetRetrieverService()
	return retrieverService.ProcessRetrieval(ctx, req)
}
