package retriever

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/formatter"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/pkg/schema"
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

	// 从注册表获取 LLM 模型配置
	llmModels := model.Registry.GetByType(model.ModelTypeLLM)
	if len(llmModels) == 0 {
		return nil, fmt.Errorf("no LLM models registered in registry")
	}

	g.Log().Infof(ctx, "Found %d LLM models for query rewrite", len(llmModels))

	// 确定重写次数，默认为3次
	rewriteAttempts := *req.RewriteAttempts
	if rewriteAttempts <= 0 {
		rewriteAttempts = 3
	}

	// 优化策略：串行执行查询重写（保证查询多样性），并发执行检索（提高速度）
	// 第一步：串行生成多个优化查询
	optimizedQueries := make([]string, 0, rewriteAttempts)

	// 配置重试机制
	retryConfig := &common.LLMRetryConfig{
		MaxRetries:    3,                      // 每次重写最多尝试3个不同的模型
		RetryDelay:    300 * time.Millisecond, // 重试延迟300ms
		ModelType:     model.ModelTypeLLM,
		ExcludeModels: []string{},
	}

	for i := 0; i < rewriteAttempts; i++ {
		// 生成优化查询消息
		optMessages, err := common.GetOptimizedQueryMessages(used, req.Query, req.KnowledgeId)
		if err != nil {
			g.Log().Errorf(ctx, "GetOptimizedQueryMessages failed at attempt %d: %v", i+1, err)
			continue
		}

		// 使用重试机制调用LLM进行查询重写
		result, err := common.RetryWithDifferentLLM(ctx, retryConfig, func(ctx context.Context, modelID string) (interface{}, error) {
			// 获取模型配置
			mc := model.Registry.Get(modelID)
			if mc == nil {
				return nil, fmt.Errorf("模型不存在: %s", modelID)
			}

			// 创建模型服务
			modelFormatter := formatter.NewOpenAIFormatter()
			modelService := model.NewModelService(mc.APIKey, mc.BaseURL, modelFormatter)

			// 调用LLM进行查询重写
			resp, err := modelService.ChatCompletion(ctx, model.ChatCompletionParams{
				ModelName:   mc.Name,
				Messages:    optMessages,
				Temperature: 0.7,
			})
			if err != nil {
				return nil, fmt.Errorf("ChatCompletion failed: %w", err)
			}

			if len(resp.Choices) == 0 {
				return nil, fmt.Errorf("ChatCompletion returned no choices")
			}

			return resp.Choices[0].Message.Content, nil
		})

		if err != nil {
			g.Log().Errorf(ctx, "Query rewrite failed at attempt %d: %v", i+1, err)
			continue
		}

		optimizedQuery := result.(string)
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
					if doc.Score > old.(*schema.Document).Score {
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
		return msg[i].Score > msg[j].Score
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
