package core

import (
	"context"
	"math"
	"sort"
	"sync"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/rerank"
	milvus "github.com/Malowking/kbgo/milvus_new_re"
	er "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// RetrieveMode 定义检索模式
type RetrieveMode string

const (
	// RetrieveModeMilvus 仅使用Milvus向量检索，按相似度排序
	RetrieveModeMilvus RetrieveMode = "milvus"
	// RetrieverModeRerank 使用Milvus检索后进行Rerank重排序（默认）
	RetrieverModeRerank RetrieveMode = "rerank"
	// RetrieverModeRRF 使用RRF (Reciprocal Rank Fusion) 混合检索
	RetrieverModeRRF RetrieveMode = "rrf"
)

type RetrieveReq struct {
	Query           string       // 检索关键词
	TopK            int          // 检索结果数量
	Score           float64      // 分数阀值(0-1范围，0表示完全不相关，1表示完全相同，一般传入0.2-0.5之间的值)
	KnowledgeId     string       // 知识库ID
	EnableRewrite   bool         // 是否启用查询重写（默认 false）
	RewriteAttempts int          // 查询重写尝试次数（默认 3，仅在 EnableRewrite=true 时生效）
	RetrieveMode    RetrieveMode // 检索模式: milvus/rerank/rrf（默认 rerank）
	optQuery        string       // 优化后的检索关键词
	excludeIDs      []string     // 要排除的 _id 列表
	rankScore       float64      // 实际使用的分数阈值（归一化后的0-1范围）
}

func (x *RetrieveReq) copy() *RetrieveReq {
	return &RetrieveReq{
		Query:           x.Query,
		TopK:            x.TopK,
		Score:           x.Score,
		KnowledgeId:     x.KnowledgeId,
		EnableRewrite:   x.EnableRewrite,
		RewriteAttempts: x.RewriteAttempts,
		RetrieveMode:    x.RetrieveMode,
		optQuery:        x.optQuery,
		excludeIDs:      x.excludeIDs,
		rankScore:       x.rankScore,
	}
}

// Retrieve 检索
func (x *Rag) Retrieve(ctx context.Context, req *RetrieveReq) (msg []*schema.Document, err error) {
	// 分数阈值直接使用，因为Milvus返回的分数已经被归一化到0-1范围
	req.rankScore = req.Score

	// 根据 EnableRewrite 参数决定是否启用查询重写
	if !req.EnableRewrite {
		// 不启用查询重写，直接使用原始查询进行检索
		req.optQuery = req.Query
		return x.retrieveDoOnce(ctx, req)
	}

	// 启用查询重写
	var (
		used        = ""          // 记录已经使用过的关键词
		relatedDocs = &sync.Map{} // 记录相关docs
	)

	rewriteModel, err := common.GetRewriteModel(ctx, nil)
	if err != nil {
		return
	}

	// 确定重写次数，默认为3次
	attempts := req.RewriteAttempts
	if attempts <= 0 {
		attempts = 3
	}

	wg := &sync.WaitGroup{}
	// 尝试N次重写关键词进行搜索
	for i := 0; i < attempts; i++ {
		question := req.Query
		var (
			optMessages    []*schema.Message
			rewriteMessage *schema.Message
		)
		optMessages, err = getOptimizedQueryMessages(used, question, req.KnowledgeId)
		if err != nil {
			return
		}
		rewriteMessage, err = rewriteModel.Generate(ctx, optMessages)
		if err != nil {
			return
		}
		optimizedQuery := rewriteMessage.Content
		used += optimizedQuery + " "

		// 为每次重写创建一个新的请求副本
		reqCopy := req.copy()
		reqCopy.optQuery = optimizedQuery

		wg.Add(1)
		go func(rq *RetrieveReq) {
			defer wg.Done()
			rDocs := make([]*schema.Document, 0)
			rDocs, err = x.retrieveDoOnce(ctx, rq)
			if err != nil {
				g.Log().Errorf(ctx, "retrieveDoOnce failed, err=%v", err)
				return
			}
			for _, doc := range rDocs {
				if old, e := relatedDocs.LoadOrStore(doc.ID, doc); e {
					// 同文档则保存较高分的结果（对于不同的optQuery，rerank可能会有不同的结果）
					if doc.Score() > old.(*schema.Document).Score() {
						relatedDocs.Store(doc.ID, doc)
					}
				}
			}
		}(reqCopy)
	}
	wg.Wait()

	// 整理需要返回的数据
	relatedDocs.Range(func(key, value any) bool {
		msg = append(msg, value.(*schema.Document))
		return true
	})
	sort.Slice(msg, func(i, j int) bool {
		return msg[i].Score() > msg[j].Score()
	})
	if len(msg) > req.TopK {
		msg = msg[:req.TopK]
	}
	return
}

func (x *Rag) retrieveDoOnce(ctx context.Context, req *RetrieveReq) (relatedDocs []*schema.Document, err error) {
	g.Log().Infof(ctx, "query: %v, retrieve_mode: %v", req.optQuery, req.RetrieveMode)

	// 根据检索模式选择不同的处理策略
	switch req.RetrieveMode {
	case RetrieveModeMilvus:
		// 模式1: 仅使用Milvus向量检索
		relatedDocs, err = x.retrieveMilvusOnly(ctx, req)
	case RetrieverModeRerank:
		// 模式2: Milvus + Rerank
		relatedDocs, err = x.retrieveWithRerank(ctx, req)
	case RetrieverModeRRF:
		// 模式3: RRF混合检索
		relatedDocs, err = x.retrieveWithRRF(ctx, req)
	default:
		// 默认使用Rerank模式
		relatedDocs, err = x.retrieveWithRerank(ctx, req)
	}

	return
}

// retrieveMilvusOnly 仅使用Milvus向量检索
func (x *Rag) retrieveMilvusOnly(ctx context.Context, req *RetrieveReq) (relatedDocs []*schema.Document, err error) {
	docs, err := x.retrieve(ctx, req)
	if err != nil {
		g.Log().Errorf(ctx, "retrieve failed, err=%v", err)
		return
	}

	// 去重
	docs = common.RemoveDuplicates(docs, func(doc *schema.Document) string {
		return doc.ID
	})

	// 按照向量相似度排序并截取 TopK
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Score() > docs[j].Score()
	})
	if len(docs) > req.TopK {
		docs = docs[:req.TopK]
	}

	// 过滤低分文档
	for _, doc := range docs {
		if doc.Score() < req.rankScore {
			g.Log().Debugf(ctx, "score less: %v, related: %v", doc.Score(), doc.Content)
			continue
		}
		relatedDocs = append(relatedDocs, doc)
	}
	return
}

// retrieveWithRerank 使用Milvus检索后进行Rerank重排序
func (x *Rag) retrieveWithRerank(ctx context.Context, req *RetrieveReq) (relatedDocs []*schema.Document, err error) {
	docs, err := x.retrieve(ctx, req)
	if err != nil {
		g.Log().Errorf(ctx, "retrieve failed, err=%v", err)
		return
	}

	// 去重
	docs = common.RemoveDuplicates(docs, func(doc *schema.Document) string {
		return doc.ID
	})

	// 使用Rerank重排序
	docs, err = rerank.NewRerank(ctx, req.optQuery, docs, req.TopK)
	if err != nil {
		g.Log().Errorf(ctx, "Rerank failed, err=%v", err)
		return
	}

	// 过滤低分文档
	for _, doc := range docs {
		if doc.Score() < req.rankScore {
			g.Log().Debugf(ctx, "score less: %v, related: %v", doc.Score(), doc.Content)
			continue
		}
		relatedDocs = append(relatedDocs, doc)
	}
	return
}

// retrieveWithRRF 使用RRF (Reciprocal Rank Fusion) 混合检索
// RRF公式: score = sum(1/(k+rank)), k通常为60
func (x *Rag) retrieveWithRRF(ctx context.Context, req *RetrieveReq) (relatedDocs []*schema.Document, err error) {
	const k = 60.0 // RRF常数

	// 1. 原始查询检索
	docs1, err := x.retrieve(ctx, req)
	if err != nil {
		g.Log().Errorf(ctx, "retrieve with original query failed, err=%v", err)
		return
	}

	// 2. 使用Rerank作为第二路召回
	docs2, err := x.retrieve(ctx, req)
	if err != nil {
		g.Log().Errorf(ctx, "retrieve for rerank failed, err=%v", err)
		return
	}
	docs2, err = rerank.NewRerank(ctx, req.optQuery, docs2, req.TopK*2)
	if err != nil {
		g.Log().Errorf(ctx, "Rerank failed, err=%v", err)
		return
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

	// 6. 截取TopK
	if len(docs) > req.TopK {
		docs = docs[:req.TopK]
	}

	// 7. 过滤低分文档
	for _, doc := range docs {
		if doc.Score() < req.rankScore {
			g.Log().Debugf(ctx, "score less: %v, related: %v", doc.Score(), doc.Content)
			continue
		}
		relatedDocs = append(relatedDocs, doc)
	}

	return
}

func (x *Rag) retrieve(ctx context.Context, req *RetrieveReq) (msg []*schema.Document, err error) {
	var filter string
	// 如果有需要排除的ID，添加到 filter 中
	if len(req.excludeIDs) > 0 {
		// 构建 id not in [...] 表达式
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

	// 动态构建 retriever
	r, err := x.BuildRetriever(ctx, collectionName)
	if err != nil {
		g.Log().Errorf(ctx, "BuildRetriever failed for collection %s, err=%v", collectionName, err)
		return nil, err
	}

	// Milvus 检索的 TopK，可以设置得比最终需要的数量大一些
	// 因为后续会经过 rerank 重新排序
	milvusTopK := req.TopK * 5 // 取5倍数量，给 rerank 更多选择空间
	if milvusTopK < 20 {
		milvusTopK = 20 // 至少取20个
	}

	// 执行检索
	var options []er.Option
	options = append(options, er.WithTopK(milvusTopK))

	// 只有在有过滤条件时才添加 filter
	if filter != "" {
		options = append(options, milvus.WithFilter(filter))
	}

	msg, err = r.Invoke(ctx, req.optQuery,
		compose.WithRetrieverOption(options...),
	)

	// 归一化Milvus的COSINE分数（0-2范围）到标准的0-1范围
	// Milvus COSINE分数含义：0=完全相反, 1=正交, 2=完全相同
	// 归一化后：0=完全相反, 0.5=正交, 1=完全相同
	for _, s := range msg {
		normalizedScore := s.Score() / 2.0
		s.WithScore(normalizedScore)
	}

	if err != nil {
		return
	}
	return
}
