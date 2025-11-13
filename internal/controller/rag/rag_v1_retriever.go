package rag

import (
	"context"
	"encoding/json"
	"sort"

	gorag "github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/Malowking/kbgo/api/rag/v1"
)

func (c *ControllerV1) Retriever(ctx context.Context, req *v1.RetrieverReq) (res *v1.RetrieverRes, err error) {
	ragSvr := rag.GetRagSvr()
	if req.TopK == 0 {
		req.TopK = 5
	}
	if req.Score == 0 {
		req.Score = 0.2 // 默认0.2，对应归一化后的分数范围
	}
	// 默认检索模式为rerank
	if req.RetrieveMode == "" {
		req.RetrieveMode = "rerank"
	}
	// 分数现在已经是0-1范围，不需要额外转换
	ragReq := &gorag.RetrieveReq{
		Query:           req.Question,
		TopK:            req.TopK,
		Score:           req.Score,
		KnowledgeId:     req.KnowledgeId,
		EnableRewrite:   req.EnableRewrite,
		RewriteAttempts: req.RewriteAttempts,
		RetrieveMode:    gorag.RetrieveMode(req.RetrieveMode),
	}
	g.Log().Infof(ctx, "ragReq: %v, EnableRewrite: %v, RewriteAttempts: %v, RetrieveMode: %v", ragReq, req.EnableRewrite, req.RewriteAttempts, req.RetrieveMode)
	msg, err := ragSvr.Retrieve(ctx, ragReq)
	if err != nil {
		return
	}
	for _, document := range msg {
		if document.MetaData != nil {
			if metadataVal, ok := document.MetaData["metadata"]; ok && metadataVal != nil {
				// 尝试将 metadata 从 JSON 字符串解析为 map
				if metadataStr, isString := metadataVal.(string); isString && metadataStr != "" {
					m := make(map[string]interface{})
					if err = json.Unmarshal([]byte(metadataStr), &m); err == nil {
						document.MetaData["metadata"] = m
					}
					// 如果解析失败，保持原值
				}
			}
		}
	}
	// eino 默认是把分高的排在两边，这里我修改
	sort.Slice(msg, func(i, j int) bool {
		return msg[i].Score() > msg[j].Score()
	})
	res = &v1.RetrieverRes{
		Document: msg,
	}
	return
}
