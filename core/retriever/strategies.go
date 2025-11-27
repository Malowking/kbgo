package retriever

import (
	"context"
	"math"
	"sort"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/rerank"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// retrieveWithRerank 使用Milvus检索后进行Rerank重排序
func retrieveWithRerank(ctx context.Context, conf *config.RetrieverConfig, req *RetrieveReq) ([]*schema.Document, error) {
	docs, err := retrieve(ctx, conf, req)
	if err != nil {
		g.Log().Errorf(ctx, "retrieve failed, err=%v", err)
		return nil, err
	}

	// 去重
	docs = common.RemoveDuplicates(docs, func(doc *schema.Document) string {
		return doc.ID
	})

	// 使用Rerank重排序，直接使用req中已设置好的TopK
	docs, err = rerank.NewRerank(ctx, req.optQuery, docs, *req.TopK)
	if err != nil {
		g.Log().Errorf(ctx, "Rerank failed, err=%v", err)
		return nil, err
	}

	// 过滤低分文档
	var relatedDocs []*schema.Document
	for _, doc := range docs {
		if doc.Score() < *req.Score {
			g.Log().Debugf(ctx, "score less: %v, related: %v", doc.Score(), doc.Content)
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
	docs2, err = rerank.NewRerank(ctx, req.optQuery, docs2, (*req.TopK)*2)
	if err != nil {
		g.Log().Errorf(ctx, "Rerank failed, err=%v", err)
		return nil, err
	}

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

		doc.WithScore(normalizedScore)
		docs = append(docs, doc)
	}

	// 5. 按RRF分数排序
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Score() > docs[j].Score()
	})

	// 6. 截取TopK，直接使用req中已设置好的TopK
	if len(docs) > *req.TopK {
		docs = docs[:*req.TopK]
	}

	// 7. 过滤低分文档
	var relatedDocs []*schema.Document
	for _, doc := range docs {
		if doc.Score() < *req.Score {
			g.Log().Debugf(ctx, "score less: %v, related: %v", doc.Score(), doc.Content)
			continue
		}
		relatedDocs = append(relatedDocs, doc)
	}

	return relatedDocs, nil
}
