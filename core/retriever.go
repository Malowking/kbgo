package core

import (
	"context"
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

type RetrieveReq struct {
	Query       string   // 检索关键词
	TopK        int      // 检索结果数量
	Score       float64  //  分数阀值(0-2, 0 完全相反，1 毫不相干，2 完全相同,一般需要传入一个大于1的数字，如1.5)
	KnowledgeId string   // 知识库ID
	optQuery    string   // 优化后的检索关键词
	excludeIDs  []string // 要排除的 _id 列表
	rankScore   float64  // 排名分数，原本的score是0-2（实际是1-2），需要在这里改成0-1
}

func (x *RetrieveReq) copy() *RetrieveReq {
	return &RetrieveReq{
		Query:       x.Query,
		TopK:        x.TopK,
		Score:       x.Score,
		KnowledgeId: x.KnowledgeId,
		optQuery:    x.optQuery,
		excludeIDs:  x.excludeIDs,
		rankScore:   x.rankScore,
	}
}

// Retrieve 检索
func (x *Rag) Retrieve(ctx context.Context, req *RetrieveReq) (msg []*schema.Document, err error) {
	var (
		used        = ""          // 记录已经使用过的关键词
		relatedDocs = &sync.Map{} // 记录相关docs
	)
	req.rankScore = req.Score
	// 大于1的需要-1
	if req.rankScore >= 1 {
		req.rankScore -= 1
	}
	rewriteModel, err := common.GetRewriteModel(ctx, nil)
	if err != nil {
		return
	}
	wg := &sync.WaitGroup{}
	// 尝试N次重写关键词进行搜索,后续可以考虑做成配置
	for i := 0; i < 3; i++ {
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
		req.optQuery = optimizedQuery
		wg.Add(1)
		go func() {
			defer wg.Done()
			rDocs := make([]*schema.Document, 0)
			rDocs, err = x.retrieveDoOnce(ctx, req.copy())
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

		}()
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
	var (
		docs   []*schema.Document
		qaDocs []*schema.Document
	)
	g.Log().Infof(ctx, "query: %v", req.optQuery)
	// 通过内容检索
	docs, err = x.retrieve(ctx, req)
	if err != nil {
		g.Log().Errorf(ctx, "retrieve failed, err=%v", err)
		return
	}
	// 通过qa检索
	qaDocs, err = x.retrieve(ctx, req)
	if err != nil {
		g.Log().Errorf(ctx, "qa retrieve failed, err=%v", err)
		return
	}
	docs = append(docs, qaDocs...)
	// 去重
	docs = common.RemoveDuplicates(docs, func(doc *schema.Document) string {
		return doc.ID
	})
	// 重排
	docs, err = rerank.NewRerank(ctx, req.optQuery, docs, req.TopK)
	if err != nil {
		g.Log().Errorf(ctx, "Rerank failed, err=%v", err)
		return
	}
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
	// 构建 Milvus filter 表达式
	// 在 Milvus 中，使用 knowledge_name 字段过滤
	filter := common.KnowledgeId + ` == "` + req.KnowledgeId + `"`

	// 如果有需要排除的ID，添加到 filter 中
	if len(req.excludeIDs) > 0 {
		// 构建 id not in [...] 表达式
		filter += " && id not in ["
		for i, id := range req.excludeIDs {
			if i > 0 {
				filter += ", "
			}
			filter += `"` + id + `"`
		}
		filter += "]"
	}

	// TODO根据是否是QA检索，选择不同的collection
	// text collection 和 qa collection 的命名规则：text_kb{id} 和 qa_kb{id}
	//collectionPrefix := "text_"
	//if qa {
	//	collectionPrefix = "qa_"
	//}

	// 从 knowledge name 推导 collection name
	// 假设 knowledge name 就是 kb id
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
	msg, err = r.Invoke(ctx, req.optQuery,
		compose.WithRetrieverOption(
			er.WithTopK(milvusTopK),
			milvus.WithFilter(filter),
		),
	)

	// Milvus 的相似度分数范围与 ES 不同，可能需要调整
	// 这里保持原有逻辑，如果分数大于1则减1
	for _, s := range msg {
		if s.Score() > 1 {
			// 本身没意义，最终分数由rerank给，这里只是为了方便测试观察
			s.WithScore(s.Score() - 1)
		}
	}

	if err != nil {
		return
	}
	return
}
