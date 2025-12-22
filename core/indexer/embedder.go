package indexer

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"time"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// VectorStoreEmbedder 向量存储嵌入器实现（增强版，支持重试和并发）
type VectorStoreEmbedder struct {
	embedding   *common.CustomEmbedder
	vectorStore vector_store.VectorStore
	modelConfig interface{} // 保存模型配置，用于提取维度信息
	configDim   int         // 配置文件中的向量维度（fallback）
}

// BatchInfo 批次信息
type BatchInfo struct {
	Index  int
	Start  int
	End    int
	Chunks []*schema.Document
	Texts  []string
}

// BatchResult 批次结果
type BatchResult struct {
	BatchIndex int
	Vectors    [][]float32
	ChunkIds   []string
	Error      error
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

// EmbedAndStore 嵌入向量并存储（增强版，支持重试和并发）
func (v *VectorStoreEmbedder) EmbedAndStore(ctx context.Context, collectionName string, chunks []*schema.Document) ([]string, error) {
	if len(chunks) == 0 {
		return []string{}, nil
	}

	// 配置参数（可以根据需要调整）
	const (
		batchSize    = 30               // 每批30个文本（避免API限制）
		concurrency  = 3                // 3个并发（避免API限流）
		maxRetries   = 5                // 最大重试次数
		initialDelay = 1 * time.Second  // 初始延迟
		maxDelay     = 30 * time.Second // 最大延迟
		multiplier   = 2.0              // 指数退避倍数
	)

	g.Log().Infof(ctx, "Starting enhanced vectorization of %d chunks (BatchSize: %d, Concurrency: %d)",
		len(chunks), batchSize, concurrency)

	// 1. 分批处理
	batches := v.createBatches(chunks, batchSize)
	g.Log().Infof(ctx, "Split into %d batches", len(batches))

	// 2. 并发处理批次
	resultChan := make(chan BatchResult, len(batches))
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	// 处理每个批次
	for _, batch := range batches {
		wg.Add(1)
		go func(b BatchInfo) {
			defer wg.Done()

			// 获取并发许可
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 处理批次
			vectors, err := v.embedTextsWithRetry(ctx, b.Texts, maxRetries, initialDelay, maxDelay, multiplier)
			if err != nil {
				resultChan <- BatchResult{
					BatchIndex: b.Index,
					Error:      errors.Newf(errors.ErrEmbeddingFailed, "batch %d failed: %v", b.Index, err),
				}
				return
			}

			// 存储到向量数据库
			chunkIds, err := v.vectorStore.InsertVectors(ctx, collectionName, b.Chunks, vectors)
			if err != nil {
				resultChan <- BatchResult{
					BatchIndex: b.Index,
					Error:      errors.Newf(errors.ErrVectorInsert, "batch %d storage failed: %v", b.Index, err),
				}
				return
			}

			resultChan <- BatchResult{
				BatchIndex: b.Index,
				Vectors:    vectors,
				ChunkIds:   chunkIds,
				Error:      nil,
			}

			g.Log().Infof(ctx, "Batch %d completed successfully, chunks: %d", b.Index, len(b.Chunks))
		}(batch)
	}

	// 等待所有批次完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 3. 收集结果
	allChunkIds := make([]string, len(chunks))
	batchResults := make([]BatchResult, len(batches))

	for result := range resultChan {
		if result.Error != nil {
			return nil, result.Error
		}
		batchResults[result.BatchIndex] = result
	}

	// 4. 按顺序组装结果
	currentIndex := 0
	for _, batch := range batches {
		result := batchResults[batch.Index]
		copy(allChunkIds[currentIndex:currentIndex+len(result.ChunkIds)], result.ChunkIds)
		currentIndex += len(result.ChunkIds)
	}

	g.Log().Infof(ctx, "Enhanced vectorization completed, total chunks: %d", len(allChunkIds))
	return allChunkIds, nil
}

// createBatches 创建批次
func (v *VectorStoreEmbedder) createBatches(chunks []*schema.Document, batchSize int) []BatchInfo {
	var batches []BatchInfo
	batchCount := int(math.Ceil(float64(len(chunks)) / float64(batchSize)))

	for i := 0; i < batchCount; i++ {
		start := i * batchSize
		end := start + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batchChunks := chunks[start:end]
		texts := make([]string, len(batchChunks))
		for j, chunk := range batchChunks {
			texts[j] = chunk.Content
		}

		batches = append(batches, BatchInfo{
			Index:  i,
			Start:  start,
			End:    end,
			Chunks: batchChunks,
			Texts:  texts,
		})
	}

	return batches
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

// embedTextsWithRetry 带重试的文本向量化
func (v *VectorStoreEmbedder) embedTextsWithRetry(ctx context.Context, texts []string, maxRetries int, initialDelay, maxDelay time.Duration, multiplier float64) ([][]float32, error) {
	var lastErr error
	delay := initialDelay

	// 获取维度
	dimensions := v.getDimension(ctx)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			g.Log().Infof(ctx, "Retrying embedding attempt %d/%d after %v delay",
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

		vectors, err := v.embedding.EmbedStrings(ctx, texts, dimensions)
		if err != nil {
			lastErr = err
			g.Log().Warningf(ctx, "Embedding attempt %d failed: %v", attempt+1, err)
			continue
		}

		return vectors, nil
	}

	return nil, errors.Newf(errors.ErrEmbeddingFailed, "embedding failed after %d retries, last error: %v", maxRetries, lastErr)
}
