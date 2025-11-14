package rag

import (
	"context"

	"github.com/Malowking/kbgo/api/rag/v1"
	"github.com/Malowking/kbgo/internal/logic/chat"
	rag2 "github.com/Malowking/kbgo/internal/logic/rag"
)

func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	// 获取检索配置
	cfg := rag2.GetRetrieverConfig()

	retriever, err := c.Retriever(ctx, &v1.RetrieverReq{
		Question:        req.Question,
		TopK:            req.TopK,
		Score:           req.Score,
		KnowledgeId:     req.KnowledgeId,
		EnableRewrite:   cfg.EnableRewrite,
		RewriteAttempts: cfg.RewriteAttempts,
		RetrieveMode:    cfg.RetrieveMode,
	})
	if err != nil {
		return
	}
	chatI := chat.GetChat()
	answer, err := chatI.GetAnswer(ctx, req.ConvID, retriever.Document, req.Question)
	if err != nil {
		return
	}
	res = &v1.ChatRes{
		Answer:     answer,
		References: retriever.Document,
	}
	return
}
