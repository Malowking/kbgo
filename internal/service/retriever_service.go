package service

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	gorag "github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/internal/logic/rag"
)

// RetrieverService 检索服务 - 提供统一的检索逻辑
type RetrieverService struct{}

// NewRetrieverService 创建检索服务
func NewRetrieverService() *RetrieverService {
	return &RetrieverService{}
}

// ProcessRetrieval 处理知识库检索
func (s *RetrieverService) ProcessRetrieval(ctx context.Context, req *v1.RetrieverReq) (*v1.RetrieverRes, error) {
	// 设置默认值
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

	// 构建内部请求
	ragReq := &gorag.RetrieveReq{
		Query:           req.Question,
		TopK:            req.TopK,
		Score:           req.Score,
		KnowledgeId:     req.KnowledgeId,
		EnableRewrite:   req.EnableRewrite,
		RewriteAttempts: req.RewriteAttempts,
		RetrieveMode:    gorag.RetrieveMode(req.RetrieveMode),
	}

	// 获取RAG服务并调用检索
	ragSvr := rag.GetRagSvr()
	msg, err := ragSvr.Retrieve(ctx, ragReq)
	if err != nil {
		return nil, err
	}

	// 处理元数据：将JSON字符串解析为map
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

	// 按分数降序排序
	sort.Slice(msg, func(i, j int) bool {
		return msg[i].Score() > msg[j].Score()
	})

	return &v1.RetrieverRes{
		Document: msg,
	}, nil
}

// GetRetrieverService 获取检索服务单例
func GetRetrieverService() *RetrieverService {
	return &RetrieverService{}
}
