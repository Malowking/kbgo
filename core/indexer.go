package core

import (
	"context"

	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/indexer"
)

// 这是一个便捷的导出函数，实际实现在 indexer 包中
func NewDocumentIndexer(ctx context.Context, conf *config.IndexerConfig) (*indexer.DocumentIndexer, error) {
	return &indexer.DocumentIndexer{
		Config:      conf,
		VectorStore: conf.VectorStore,
	}, nil
}
