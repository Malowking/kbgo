package vector_store

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/os/gctx"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/gogf/gf/v2/frame/g"
)

var (
	once         sync.Once
	vectorClient VectorStore
	initError    error
)

// GetVectorStore returns the singleton vector database client
func GetVectorStore() (VectorStore, error) {
	once.Do(func() {
		ctx := gctx.New()
		vectorClient, initError = initializeVectorStore(ctx)
	})
	return vectorClient, initError
}

// initializeVectorStore determines which client to use based on configuration
func initializeVectorStore(ctx context.Context) (VectorStore, error) {
	// Read the vector database type from configuration
	dbType := g.Cfg().MustGet(ctx, "vectorStore.type", "milvus").String()

	g.Log().Infof(ctx, "Initializing vector store with type: %s", dbType)

	switch dbType {
	case "milvus":
		store, err := InitializeMilvusStore(ctx)
		if err != nil {
			return nil, errors.Newf(errors.ErrVectorStoreInit, "failed to initialize Milvus vector store: %v", err)
		}
		g.Log().Info(ctx, "Milvus vector store initialized successfully")
		return store, nil
	case "pgvector":
		store, err := InitializePostgresStore(ctx)
		if err != nil {
			return nil, errors.Newf(errors.ErrVectorStoreInit, "failed to initialize PostgreSQL vector store: %v", err)
		}
		g.Log().Info(ctx, "PostgreSQL vector store initialized successfully")
		return store, nil
	//case "pinecone":
	//	return initializePineconeClient(ctx)
	//case "weaviate":
	//	return initializeWeaviateClient(ctx)
	default:
		return nil, errors.Newf(errors.ErrInvalidParameter, "unsupported vector database type: %s. Supported types: milvus, postgresql", dbType)
	}
}
