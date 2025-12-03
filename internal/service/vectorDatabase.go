package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

var (
	once         sync.Once
	vectorClient vector_store.VectorStore
	initError    error
)

// GetVectorStore returns the singleton vector database client
func GetVectorStore() (vector_store.VectorStore, error) {
	once.Do(func() {
		ctx := gctx.New()
		vectorClient, initError = initializeVectorStore(ctx)
	})
	return vectorClient, initError
}

// initializeVectorStore determines which client to use based on configuration
func initializeVectorStore(ctx context.Context) (vector_store.VectorStore, error) {
	// Read the vector database type from configuration
	dbType := g.Cfg().MustGet(ctx, "vectorStore.type", "milvus").String()

	g.Log().Infof(ctx, "Initializing vector store with type: %s", dbType)

	switch dbType {
	case "milvus":
		store, err := vector_store.InitializeMilvusStore(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Milvus vector store: %w", err)
		}
		g.Log().Info(ctx, "Milvus vector store initialized successfully")
		return store, nil
	case "pgvector":
		store, err := vector_store.InitializePostgresStore(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize PostgreSQL vector store: %w", err)
		}
		g.Log().Info(ctx, "PostgreSQL vector store initialized successfully")
		return store, nil
	//case "pinecone":
	//	return initializePineconeClient(ctx)
	//case "weaviate":
	//	return initializeWeaviateClient(ctx)
	default:
		return nil, fmt.Errorf("unsupported vector database type: %s. Supported types: milvus, postgresql", dbType)
	}
}
