package core

import (
	"context"

	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/indexer"
)

func NewDocumentIndexer(ctx context.Context, conf *config.IndexerConfig) (*indexer.DocumentIndexer, error) {
	return &indexer.DocumentIndexer{
		Config:      conf,
		VectorStore: conf.VectorStore,
	}, nil
}
