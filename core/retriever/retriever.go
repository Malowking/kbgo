package retriever

import (
	"context"
	"sort"
	"sync"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// Retrieve 执行检索（主方法）
func Retrieve(ctx context.Context, conf *config.RetrieverConfig, req *RetrieveReq) ([]*schema.Document, error) {
	// 使用配置中的默认值填充请求中未提供的参数
	if req.TopK == nil {
		req.TopK = &conf.TopK
	}

	if req.Score == nil {
		req.Score = &conf.Score
	}

	if req.EnableRewrite == nil {
		req.EnableRewrite = &conf.EnableRewrite
	}

	if req.RewriteAttempts == nil {
		req.RewriteAttempts = &conf.RewriteAttempts
	}

	if req.RetrieveMode == nil {
		defaultMode := RetrieveMode(conf.RetrieveMode)
		req.RetrieveMode = &defaultMode
	}

	// 根据 EnableRewrite 参数决定是否启用查询重写
	if !*req.EnableRewrite {
		// 不启用查询重写，直接使用原始查询进行检索
		req.optQuery = req.Query
		return retrieveDoOnce(ctx, conf, req)
	}

	// 启用查询重写
	var (
		relatedDocs = &sync.Map{} // 记录相关docs
		used        = ""          // 记录已经使用过的关键词
	)

	rewriteModel, err := common.GetRewriteModel(ctx, nil)
	if err != nil {
		return nil, err
	}

	// 确定重写次数，默认为3次
	rewriteAttempts := *req.RewriteAttempts
	if rewriteAttempts <= 0 {
		rewriteAttempts = 3
	}

	// 优化策略：串行执行查询重写（保证查询多样性），并发执行检索（提高速度）
	// 第一步：串行生成多个优化查询
	optimizedQueries := make([]string, 0, rewriteAttempts)
	for i := 0; i < rewriteAttempts; i++ {
		// 生成优化查询消息
		optMessages, err := common.GetOptimizedQueryMessages(used, req.Query, req.KnowledgeId)
		if err != nil {
			g.Log().Errorf(ctx, "GetOptimizedQueryMessages failed at attempt %d: %v", i+1, err)
			continue
		}

		// 调用LLM进行查询重写
		rewriteMessage, err := rewriteModel.Generate(ctx, optMessages)
		if err != nil {
			g.Log().Errorf(ctx, "rewriteModel.Generate failed at attempt %d: %v", i+1, err)
			continue
		}
		optimizedQuery := rewriteMessage.Content
		used += optimizedQuery + " "

		g.Log().Infof(ctx, "Rewrite attempt %d: %s", i+1, optimizedQuery)
		optimizedQueries = append(optimizedQueries, optimizedQuery)
	}

	// 如果没有成功生成任何优化查询，使用原始查询
	if len(optimizedQueries) == 0 {
		g.Log().Warningf(ctx, "No optimized queries generated, using original query")
		optimizedQueries = append(optimizedQueries, req.Query)
	}

	// 第二步：并发执行所有查询的检索
	wg := &sync.WaitGroup{}
	for _, optimizedQuery := range optimizedQueries {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()

			// 使用优化后的查询进行检索
			reqCopy := req.Copy()
			reqCopy.optQuery = query

			rDocs, err := retrieveDoOnce(ctx, conf, reqCopy)
			if err != nil {
				g.Log().Errorf(ctx, "retrieveDoOnce failed for query '%s': %v", query, err)
				return
			}

			// 合并检索结果
			for _, doc := range rDocs {
				if old, e := relatedDocs.LoadOrStore(doc.ID, doc); e {
					// 同文档则保存较高分的结果（对于不同的optQuery，rerank可能会有不同的结果）
					if doc.Score() > old.(*schema.Document).Score() {
						relatedDocs.Store(doc.ID, doc)
					}
				}
			}
		}(optimizedQuery)
	}
	wg.Wait()

	// 整理需要返回的数据
	var msg []*schema.Document
	relatedDocs.Range(func(key, value any) bool {
		msg = append(msg, value.(*schema.Document))
		return true
	})
	sort.Slice(msg, func(i, j int) bool {
		return msg[i].Score() > msg[j].Score()
	})
	if len(msg) > *req.TopK {
		msg = msg[:*req.TopK]
	}
	return msg, nil
}

// retrieveDoOnce 单次检索分发
func retrieveDoOnce(ctx context.Context, conf *config.RetrieverConfig, req *RetrieveReq) ([]*schema.Document, error) {
	g.Log().Infof(ctx, "query: %v, retrieve_mode: %v", req.optQuery, *req.RetrieveMode)

	// 根据检索模式选择不同的处理策略
	switch *req.RetrieveMode {
	case RetrieveModeMilvus:
		// 模式1: 仅使用Milvus向量检索，直接调用VectorStore的方法
		return conf.VectorStore.VectorSearchOnly(ctx, conf, req.optQuery, req.KnowledgeId, *req.TopK, *req.Score)
	case RetrieveModeRerank:
		// 模式2: Milvus + Rerank
		return retrieveWithRerank(ctx, conf, req)
	case RetrieveModeRRF:
		// 模式3: RRF混合检索
		return retrieveWithRRF(ctx, conf, req)
	default:
		// 默认使用Rerank模式
		return retrieveWithRerank(ctx, conf, req)
	}
}
