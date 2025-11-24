package indexer

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/indexer/vector_store"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// VectorStoreEmbedder 向量存储嵌入器实现
type VectorStoreEmbedder struct {
	embedding   embedding.Embedder
	vectorStore vector_store.VectorStore
}

// NewVectorStoreEmbedder 创建向量存储嵌入器
func NewVectorStoreEmbedder(ctx context.Context, conf *config.Config, vectorStore vector_store.VectorStore) (*VectorStoreEmbedder, error) {
	// Create embedding instance
	embeddingIns, err := common.NewEmbedding(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding instance: %w", err)
	}

	return &VectorStoreEmbedder{
		embedding:   embeddingIns,
		vectorStore: vectorStore,
	}, nil
}

// EmbedAndStore 嵌入向量并存储
func (v *VectorStoreEmbedder) EmbedAndStore(ctx context.Context, collectionName string, chunks []*schema.Document) ([]string, error) {
	if len(chunks) == 0 {
		return []string{}, nil
	}

	// 1. Extract text content
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// 2. Vectorization
	g.Log().Infof(ctx, "Starting vectorization of %d chunks", len(texts))
	vectors, err := v.embedding.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("failed to embed texts: %w", err)
	}
	g.Log().Infof(ctx, "Vectorization completed, obtained %d vectors", len(vectors))

	// 3. Insert into vector database
	chunkIds, err := v.vectorStore.InsertVectors(ctx, collectionName, chunks, vectors)
	if err != nil {
		return nil, fmt.Errorf("failed to insert vectors: %w", err)
	}
	return chunkIds, nil
}
