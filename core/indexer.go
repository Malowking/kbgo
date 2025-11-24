package core

import (
	"context"

	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/indexer"
)

// NewDocumentIndexer 创建文档索引服务
// 这是一个便捷的导出函数，实际实现在 indexer 包中
func NewDocumentIndexer(ctx context.Context, conf *config.Config) (*indexer.DocumentIndexer, error) {
	return indexer.NewDocumentIndexer(ctx, conf)
}
