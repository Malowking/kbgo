package indexer

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// VectorStoreEmbedder 向量存储嵌入器实现
type VectorStoreEmbedder struct {
	embedding   *common.CustomEmbedder
	vectorStore vector_store.VectorStore
	modelConfig interface{} // 保存模型配置，用于提取维度信息
	configDim   int         // 配置文件中的向量维度（fallback）
}

// NewVectorStoreEmbedder 创建向量存储嵌入器
func NewVectorStoreEmbedder(ctx context.Context, conf common.EmbeddingConfig, vectorStore vector_store.VectorStore, modelConfig interface{}, configDim int) (*VectorStoreEmbedder, error) {
	// Create embedding instance
	embeddingIns, err := common.NewEmbedding(ctx, conf)
	if err != nil {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "failed to create embedding instance: %v", err)
	}

	return &VectorStoreEmbedder{
		embedding:   embeddingIns,
		vectorStore: vectorStore,
		modelConfig: modelConfig,
		configDim:   configDim,
	}, nil
}

// EmbedAndStore 嵌入向量并存储（单条并发版，支持重试）
func (v *VectorStoreEmbedder) EmbedAndStore(ctx context.Context, collectionName string, chunks []*schema.Document) ([]string, error) {
	if len(chunks) == 0 {
		return []string{}, nil
	}

	// 配置参数
	const (
		concurrency  = 10               // 10个并发（单条处理）
		maxRetries   = 5                // 最大重试次数
		initialDelay = 1 * time.Second  // 初始延迟
		maxDelay     = 30 * time.Second // 最大延迟
		multiplier   = 2.0              // 指数退避倍数
	)

	g.Log().Infof(ctx, "Starting single-chunk concurrent vectorization of %d chunks (Concurrency: %d)",
		len(chunks), concurrency)

	// 结果通道和并发控制
	type ChunkResult struct {
		Index   int
		ChunkID string
		Vector  []float32
		Error   error
	}

	resultChan := make(chan ChunkResult, len(chunks))
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	// 并发处理每个chunk
	for i, chunk := range chunks {
		wg.Add(1)
		go func(index int, ch *schema.Document) {
			defer wg.Done()

			// 获取并发许可
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 单条embedding调用（带重试）
			vector, err := v.embedSingleChunkWithRetry(ctx, ch.Content, maxRetries, initialDelay, maxDelay, multiplier)
			if err != nil {
				resultChan <- ChunkResult{
					Index: index,
					Error: errors.Newf(errors.ErrEmbeddingFailed, "chunk %d embedding failed: %v", index, err),
				}
				return
			}

			// 单条存储到向量数据库
			chunkIds, err := v.vectorStore.InsertVectors(ctx, collectionName, []*schema.Document{ch}, [][]float32{vector})
			if err != nil {
				resultChan <- ChunkResult{
					Index: index,
					Error: errors.Newf(errors.ErrVectorInsert, "chunk %d storage failed: %v", index, err),
				}
				return
			}

			if len(chunkIds) != 1 {
				resultChan <- ChunkResult{
					Index: index,
					Error: errors.Newf(errors.ErrVectorInsert, "chunk %d: expected 1 chunkID, got %d", index, len(chunkIds)),
				}
				return
			}

			resultChan <- ChunkResult{
				Index:   index,
				ChunkID: chunkIds[0],
				Vector:  vector,
				Error:   nil,
			}

			if (index+1)%10 == 0 || index == len(chunks)-1 {
				g.Log().Infof(ctx, "Progress: %d/%d chunks completed", index+1, len(chunks))
			}
		}(i, chunk)
	}

	// 等待所有chunk完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果（按索引顺序）
	results := make([]ChunkResult, len(chunks))
	for result := range resultChan {
		if result.Error != nil {
			return nil, result.Error
		}
		results[result.Index] = result
	}

	// 提取chunkIds（保持顺序）
	allChunkIds := make([]string, len(chunks))
	for i, result := range results {
		allChunkIds[i] = result.ChunkID
	}

	g.Log().Infof(ctx, "Single-chunk concurrent vectorization completed, total chunks: %d", len(allChunkIds))
	return allChunkIds, nil
}

// embedSingleChunkWithRetry
func (v *VectorStoreEmbedder) embedSingleChunkWithRetry(ctx context.Context, text string, maxRetries int, initialDelay, maxDelay time.Duration, multiplier float64) ([]float32, error) {
	var lastErr error
	delay := initialDelay

	// 获取维度
	dimensions := v.getDimension(ctx)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			g.Log().Infof(ctx, "Retrying single chunk embedding attempt %d/%d after %v delay",
				attempt, maxRetries, delay)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// 指数退避
				delay = time.Duration(float64(delay) * multiplier)
				if delay > maxDelay {
					delay = maxDelay
				}
			}
		}

		// 单条调用embedding API
		vectors, err := v.embedding.EmbedStrings(ctx, []string{text}, dimensions)
		if err != nil {
			lastErr = err
			g.Log().Warningf(ctx, "Single chunk embedding attempt %d failed: %v", attempt+1, err)
			continue
		}

		if len(vectors) != 1 {
			lastErr = errors.Newf(errors.ErrEmbeddingFailed, "expected 1 vector, got %d", len(vectors))
			g.Log().Warningf(ctx, "Single chunk embedding attempt %d failed: %v", attempt+1, lastErr)
			continue
		}

		return vectors[0], nil
	}

	return nil, errors.Newf(errors.ErrEmbeddingFailed, "single chunk embedding failed after %d retries, last error: %v", maxRetries, lastErr)
}

// getDimension 获取embedding维度
// 1. 首先尝试从模型配置的extra字段中解析dimension
// 2. 如果没有，使用配置文件中的dim作为fallback
func (v *VectorStoreEmbedder) getDimension(ctx context.Context) int {
	// 尝试从模型配置的extra字段中提取dimension
	if v.modelConfig != nil {
		// 尝试将modelConfig转换为map类型
		if configMap, ok := v.modelConfig.(map[string]any); ok {
			if extra, exists := configMap["Extra"]; exists {
				if extraMap, ok := extra.(map[string]any); ok {
					if dim, exists := extraMap["dimension"]; exists {
						if dimInt, ok := dim.(int); ok {
							return dimInt
						}
						if dimFloat, ok := dim.(float64); ok {
							return int(dimFloat)
						}
					}
				} else if extraStr, ok := extra.(string); ok && extraStr != "" {
					// 尝试解析字符串形式的JSON
					var extraMap map[string]any
					if err := json.Unmarshal([]byte(extraStr), &extraMap); err == nil {
						if dim, exists := extraMap["dimension"]; exists {
							if dimFloat, ok := dim.(float64); ok {
								return int(dimFloat)
							}
						}
					}
				}
			}
		}
	}

	// Fallback：使用配置文件中的dim
	if v.configDim > 0 {
		return v.configDim
	}

	// 默认值
	g.Log().Warningf(ctx, "No dimension found in model config or config file, using default: 1024")
	return 1024
}
