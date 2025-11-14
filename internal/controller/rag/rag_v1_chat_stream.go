package rag

import (
	"context"

	v1 "github.com/Malowking/kbgo/api/rag/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/chat"
	rag2 "github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ChatStream 流式输出接口
func (c *ControllerV1) ChatStream(ctx context.Context, req *v1.ChatStreamReq) (res *v1.ChatStreamRes, err error) {
	var streamReader *schema.StreamReader[*schema.Message]
	// 获取检索配置
	cfg := rag2.GetRetrieverConfig()

	// 获取检索结果
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
		g.Log().Error(ctx, err)
		return
	}
	// 获取Chat实例
	chatI := chat.GetChat()
	// 获取流式响应
	streamReader, err = chatI.GetAnswerStream(ctx, req.ConvID, retriever.Document, req.Question)
	if err != nil {
		g.Log().Error(ctx, err)
		return &v1.ChatStreamRes{}, nil
	}
	defer streamReader.Close()
	err = common.SteamResponse(ctx, streamReader, retriever.Document)
	if err != nil {
		g.Log().Error(ctx, err)
		return
	}
	return &v1.ChatStreamRes{}, nil
}
