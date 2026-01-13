package retriever

import (
	"context"

	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// retrieve 执行底层的向量检索
func retrieve(ctx context.Context, conf *config.RetrieverConfig, req *RetrieveReq) ([]*schema.Document, error) {
	var filter string
	// 如果有需要排除的ID，添加到 filter 中
	if len(req.excludeIDs) > 0 {
		filter = "id not in ["
		for i, id := range req.excludeIDs {
			if i > 0 {
				filter += ", "
			}
			filter += `"` + id + `"`
		}
		filter += "]"
	}

	// knowledge name == collection name
	collectionName := req.KnowledgeId

	// 使用配置中的VectorStore
	vectorStore := conf.VectorStore

	// 使用通用的 NewRetriever 方法
	r, err := vectorStore.NewRetriever(ctx, collectionName)
	if err != nil {
		g.Log().Errorf(ctx, "failed to create retriever for collection %s, err=%v", collectionName, err)
		return nil, err
	}

	// 获取 TopK 值
	topK := conf.TopK
	if req.TopK != nil {
		topK = *req.TopK
	}

	// 因为后续会经过 rerank 重新排序，所以增大TopK
	realTopK := topK * 3 // 取3倍数量，给 rerank 更多选择空间
	if realTopK < 15 {
		realTopK = 15 // 至少取15个
	}

	// 执行检索
	var options []vector_store.Option
	options = append(options, vector_store.WithTopK(realTopK))

	// 添加分数阈值选项
	if req.Score != nil {
		options = append(options, vector_store.WithScoreThreshold(*req.Score))
	}

	// 只有在有过滤条件时才添加 filter
	if filter != "" {
		options = append(options, vector_store.WithFilter(filter))
	}

	msg, err := r.Retrieve(ctx, req.optQuery, options...)
	if err != nil {
		return nil, err
	}

	// 归一化COSINE分数
	for _, s := range msg {
		normalizedScore := s.Score / 2.0
		s.Score = normalizedScore
	}

	return msg, nil
}
