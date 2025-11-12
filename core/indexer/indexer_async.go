package indexer

import (
	"context"

	"github.com/Malowking/kbgo/core/config"
	"github.com/cloudwego/eino/components/indexer"
)

// newAsyncIndexer creates a new async Milvus indexer for the specified collection
// This is simply an alias to the regular newIndexer for now
func newAsyncIndexer(ctx context.Context, conf *config.Config, collectionName string) (idr indexer.Indexer, err error) {
	return newIndexer(ctx, conf, collectionName)
}
