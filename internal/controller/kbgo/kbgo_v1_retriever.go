package kbgo

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/retriever"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) Retriever(ctx context.Context, req *v1.RetrieverReq) (res *v1.RetrieverRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "Retriever request received - Question: %s, RerankModelID: %s, TopK: %d, Score: %f, KnowledgeId: %s, EnableRewrite: %v, RewriteAttempts: %d, RetrieveMode: %s",
		req.Question, req.RerankModelID, req.TopK, req.Score, req.KnowledgeId, req.EnableRewrite, req.RewriteAttempts, req.RetrieveMode)

	g.Log().Infof(ctx, "Received retriever request: %+v", req)

	// 直接调用 logic 层的 ProcessRetrieval 函数
	res, err = retriever.ProcessRetrieval(ctx, req)
	if err != nil {
		return res, err
	}

	// 转换文档中的图片URL为可访问的代理URL
	r := g.RequestFromCtx(ctx)
	if r != nil {
		baseURL := common.GetBaseURL(r.Host, r.URL.Scheme, map[string]string{
			"X-Forwarded-Host":  r.Header.Get("X-Forwarded-Host"),
			"X-Forwarded-Proto": r.Header.Get("X-Forwarded-Proto"),
		})
		common.ConvertImageURLsInDocuments(res.Document, baseURL)
	}

	return res, nil
}
