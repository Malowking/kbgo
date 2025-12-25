package retriever

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// convertToRerankDocs 将 schema.Document 转换为 common.RerankDocument
func convertToRerankDocs(docs []*schema.Document) []common.RerankDocument {
	result := make([]common.RerankDocument, len(docs))
	for i, doc := range docs {
		result[i] = common.RerankDocument{
			ID:      doc.ID,
			Content: doc.Content,
			Score:   float64(doc.Score), // Convert float32 to float64 for reranker
		}
	}
	return result
}

// convertFromRerankDocs 将 common.RerankDocument 转换回 schema.Document
func convertFromRerankDocs(rerankDocs []common.RerankDocument, originalDocs []*schema.Document) []*schema.Document {
	// 创建一个映射，快速查找原始文档
	docMap := make(map[string]*schema.Document)
	for _, doc := range originalDocs {
		docMap[doc.ID] = doc
	}

	result := make([]*schema.Document, 0, len(rerankDocs))
	for _, rerankDoc := range rerankDocs {
		if originalDoc, exists := docMap[rerankDoc.ID]; exists {
			// 复制原始文档并更新分数
			doc := originalDoc
			doc.Score = float32(rerankDoc.Score) // Convert back to float32
			result = append(result, doc)
		}
	}
	return result
}

// retrieveWithRerank 使用普通向量检索后进行Rerank重排序
// 当 RerankWeight < 1.0 时，混合使用Rerank和BM25关键词检索
func retrieveWithRerank(ctx context.Context, conf *config.RetrieverConfig, req *RetrieveReq) ([]*schema.Document, error) {
	startTime := time.Now()

	docs, err := retrieve(ctx, conf, req)
	if err != nil {
		g.Log().Errorf(ctx, "retrieve failed, err=%v", err)
		return nil, err
	}

	// 去重
	docs = common.RemoveDuplicates(docs, func(doc *schema.Document) string {
		return doc.ID
	})

	g.Log().Infof(ctx, "Retrieved %d documents before rerank", len(docs))

	// 检查是否需要混合BM25（当权重不为1时）
	rerankWeight := *req.RerankWeight
	bm25Weight := 1.0 - rerankWeight

	// 如果权重为1.0，则纯使用rerank（原有逻辑）
	if rerankWeight >= 0.9999 {
		return retrieveWithPureRerank(ctx, conf, req, docs, startTime)
	}

	// 如果权重为0.0，则纯使用BM25
	if rerankWeight <= 0.0001 {
		return retrieveWithPureBM25(ctx, req, docs, startTime)
	}

	// 混合模式：同时计算Rerank和BM25分数，按权重融合
	g.Log().Infof(ctx, "Using hybrid retrieval: rerank_weight=%.2f, bm25_weight=%.2f", rerankWeight, bm25Weight)

	// 1. 执行Rerank
	reranker, err := common.NewReranker(ctx, conf)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to create reranker, err=%v", err)
		return nil, err
	}

	rerankDocs := convertToRerankDocs(docs)
	subChunkConfig := common.DefaultSubChunkConfig()
	subChunkConfig.AggregateStrategy = common.AggregateStrategyMax

	g.Log().Infof(ctx, "Starting sub-chunk parallel rerank with config: size=%d, overlap=%d, strategy=%s",
		subChunkConfig.SubChunkSize, subChunkConfig.OverlapSize, subChunkConfig.AggregateStrategy)

	rerankResults, err := reranker.RerankWithSubChunks(ctx, req.optQuery, rerankDocs, len(docs), subChunkConfig)
	if err != nil {
		g.Log().Errorf(ctx, "RerankWithSubChunks failed, err=%v", err)
		return nil, err
	}

	// 2. 执行BM25关键词检索
	bm25Docs := make([]common.BM25Document, len(docs))
	for i, doc := range docs {
		bm25Docs[i] = common.BM25Document{
			ID:      doc.ID,
			Content: doc.Content,
		}
	}

	bm25Scorer := common.NewBM25Scorer(bm25Docs, common.DefaultBM25Parameters())
	bm25Results := bm25Scorer.Score(req.optQuery)

	// 归一化BM25分数
	bm25Results = common.NormalizeBM25Scores(bm25Results)

	g.Log().Infof(ctx, "BM25 scoring completed for %d documents", len(bm25Results))

	// 3. 创建分数映射
	rerankScoreMap := make(map[string]float64)
	for _, doc := range rerankResults {
		rerankScoreMap[doc.ID] = doc.Score
	}

	bm25ScoreMap := make(map[string]float64)
	for _, doc := range bm25Results {
		bm25ScoreMap[doc.ID] = doc.Score
	}

	// 4. 混合分数
	for _, doc := range docs {
		rerankScore := rerankScoreMap[doc.ID]
		bm25Score := bm25ScoreMap[doc.ID]

		// 加权融合：hybridScore = rerankWeight * rerankScore + bm25Weight * bm25Score
		hybridScore := rerankWeight*rerankScore + bm25Weight*bm25Score
		doc.Score = float32(hybridScore)

		g.Log().Debugf(ctx, "Doc %s: rerank=%.4f, bm25=%.4f, hybrid=%.4f",
			doc.ID[:8], rerankScore, bm25Score, hybridScore)
	}

	// 5. 按混合分数排序
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Score > docs[j].Score
	})

	// 6. 截取TopK
	if len(docs) > *req.TopK {
		docs = docs[:*req.TopK]
	}

	// 7. 过滤低分文档
	var relatedDocs []*schema.Document
	for _, doc := range docs {
		if doc.Score < float32(*req.Score) {
			continue
		}
		relatedDocs = append(relatedDocs, doc)
	}

	elapsed := time.Since(startTime)
	g.Log().Infof(ctx, "Hybrid retrieval completed in %v, returned %d documents", elapsed, len(relatedDocs))

	return relatedDocs, nil
}

// retrieveWithPureRerank 纯Rerank检索（原有逻辑）
func retrieveWithPureRerank(ctx context.Context, conf *config.RetrieverConfig, req *RetrieveReq, docs []*schema.Document, startTime time.Time) ([]*schema.Document, error) {
	// 创建 rerank 客户端
	reranker, err := common.NewReranker(ctx, conf)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to create reranker, err=%v", err)
		return nil, err
	}

	// 转换文档格式
	rerankDocs := convertToRerankDocs(docs)

	// 使用子切片滑窗并行 Rerank（新的优化方案）
	subChunkConfig := common.DefaultSubChunkConfig()
	subChunkConfig.AggregateStrategy = common.AggregateStrategyMax

	g.Log().Infof(ctx, "Starting sub-chunk parallel rerank with config: size=%d, overlap=%d, strategy=%s",
		subChunkConfig.SubChunkSize, subChunkConfig.OverlapSize, subChunkConfig.AggregateStrategy)

	rerankResults, err := reranker.RerankWithSubChunks(ctx, req.optQuery, rerankDocs, *req.TopK, subChunkConfig)
	if err != nil {
		g.Log().Errorf(ctx, "RerankWithSubChunks failed, err=%v", err)
		return nil, err
	}

	// 转换回 schema.Document
	docs = convertFromRerankDocs(rerankResults, docs)

	// 过滤低分文档
	var relatedDocs []*schema.Document
	for _, doc := range docs {
		if doc.Score < float32(*req.Score) {
			continue
		}
		relatedDocs = append(relatedDocs, doc)
	}

	elapsed := time.Since(startTime)
	g.Log().Infof(ctx, "Pure rerank completed in %v, returned %d documents", elapsed, len(relatedDocs))

	return relatedDocs, nil
}

// retrieveWithPureBM25 纯BM25关键词检索
func retrieveWithPureBM25(ctx context.Context, req *RetrieveReq, docs []*schema.Document, startTime time.Time) ([]*schema.Document, error) {
	// 转换为BM25文档格式
	bm25Docs := make([]common.BM25Document, len(docs))
	for i, doc := range docs {
		bm25Docs[i] = common.BM25Document{
			ID:      doc.ID,
			Content: doc.Content,
		}
	}

	// 创建BM25评分器
	bm25Scorer := common.NewBM25Scorer(bm25Docs, common.DefaultBM25Parameters())
	bm25Results := bm25Scorer.Score(req.optQuery)

	// 归一化BM25分数
	bm25Results = common.NormalizeBM25Scores(bm25Results)

	g.Log().Infof(ctx, "BM25 scoring completed for %d documents", len(bm25Results))

	// 创建BM25分数映射
	bm25ScoreMap := make(map[string]float64)
	for _, doc := range bm25Results {
		bm25ScoreMap[doc.ID] = doc.Score
	}

	// 更新文档分数
	for _, doc := range docs {
		doc.Score = float32(bm25ScoreMap[doc.ID])
	}

	// 按BM25分数排序
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Score > docs[j].Score
	})

	// 截取TopK
	if len(docs) > *req.TopK {
		docs = docs[:*req.TopK]
	}

	// 过滤低分文档
	var relatedDocs []*schema.Document
	for _, doc := range docs {
		if doc.Score < float32(*req.Score) {
			continue
		}
		relatedDocs = append(relatedDocs, doc)
	}
	return relatedDocs, nil
}

// retrieveWithRRF 使用RRF (Reciprocal Rank Fusion) 混合检索
// RRF公式: score = sum(1/(k+rank)), k通常为60
func retrieveWithRRF(ctx context.Context, conf *config.RetrieverConfig, req *RetrieveReq) ([]*schema.Document, error) {
	const k = 60.0 // RRF常数
	startTime := time.Now()

	// 1. 原始查询检索
	docs1, err := retrieve(ctx, conf, req)
	if err != nil {
		g.Log().Errorf(ctx, "retrieve with original query failed, err=%v", err)
		return nil, err
	}

	// 2. 使用Rerank作为第二路召回
	docs2, err := retrieve(ctx, conf, req)
	if err != nil {
		g.Log().Errorf(ctx, "retrieve for rerank failed, err=%v", err)
		return nil, err
	}

	// 创建 rerank 客户端
	reranker, err := common.NewReranker(ctx, conf)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to create reranker, err=%v", err)
		return nil, err
	}

	// 转换文档格式并执行子切片滑窗并行 rerank
	rerankDocs2 := convertToRerankDocs(docs2)

	// 使用子切片滑窗并行 Rerank（新的优化方案）
	subChunkConfig := common.DefaultSubChunkConfig()
	subChunkConfig.AggregateStrategy = common.AggregateStrategyMax

	g.Log().Infof(ctx, "RRF: Starting sub-chunk parallel rerank for second path")

	rerankResults2, err := reranker.RerankWithSubChunks(ctx, req.optQuery, rerankDocs2, (*req.TopK)*2, subChunkConfig)
	if err != nil {
		g.Log().Errorf(ctx, "RerankWithSubChunks failed, err=%v", err)
		return nil, err
	}
	docs2 = convertFromRerankDocs(rerankResults2, docs2)

	// 3. RRF融合
	rrfScores := make(map[string]float64) // docID -> RRF score
	docMap := make(map[string]*schema.Document)

	// 计算第一路检索的RRF分数
	for rank, doc := range docs1 {
		rrfScore := 1.0 / (k + float64(rank+1))
		rrfScores[doc.ID] = rrfScore
		docMap[doc.ID] = doc
	}

	// 计算第二路检索的RRF分数并累加
	for rank, doc := range docs2 {
		rrfScore := 1.0 / (k + float64(rank+1))
		rrfScores[doc.ID] += rrfScore
		if _, exists := docMap[doc.ID]; !exists {
			docMap[doc.ID] = doc
		}
	}

	// 4. 将RRF分数转换为文档列表
	var docs []*schema.Document
	for docID, doc := range docMap {
		// 归一化RRF分数到0-1范围
		// 最大可能分数是 2/(k+1) (两路都排第一)
		maxPossibleScore := 2.0 / (k + 1.0)
		normalizedScore := rrfScores[docID] / maxPossibleScore
		normalizedScore = math.Min(normalizedScore, 1.0) // 确保不超过1

		doc.Score = float32(normalizedScore) // Convert to float32
		docs = append(docs, doc)
	}

	// 5. 按RRF分数排序
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Score > docs[j].Score
	})

	// 6. 截取TopK，直接使用req中已设置好的TopK
	if len(docs) > *req.TopK {
		docs = docs[:*req.TopK]
	}

	// 7. 过滤低分文档
	var relatedDocs []*schema.Document
	for _, doc := range docs {
		if doc.Score < float32(*req.Score) {
			continue
		}
		relatedDocs = append(relatedDocs, doc)
	}

	elapsed := time.Since(startTime)
	g.Log().Infof(ctx, "RRF completed in %v, returned %d documents", elapsed, len(relatedDocs))

	return relatedDocs, nil
}
