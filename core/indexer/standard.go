package indexer

import (
	"context"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// StandardDocumentLoader 标准文档加载器实现
type StandardDocumentLoader struct {
	loader document.Loader
}

// NewStandardDocumentLoader 创建标准文档加载器
func NewStandardDocumentLoader(ctx context.Context) (*StandardDocumentLoader, error) {
	loader, err := Loader(ctx)
	if err != nil {
		return nil, err
	}
	return &StandardDocumentLoader{
		loader: loader,
	}, nil
}

// Load 加载文档
func (d *StandardDocumentLoader) Load(ctx context.Context, filePath string) ([]*schema.Document, error) {
	return d.loader.Load(ctx, document.Source{URI: filePath})
}

// StandardDocumentTransformer 标准文档转换器实现
type StandardDocumentTransformer struct {
	transformer document.Transformer
}

// NewStandardDocumentTransformer 创建标准文档转换器
func NewStandardDocumentTransformer(ctx context.Context, chunkSize, overlapSize int, separator string) (*StandardDocumentTransformer, error) {
	transformer, err := NewTransformer(ctx, chunkSize, overlapSize, separator)
	if err != nil {
		return nil, err
	}

	return &StandardDocumentTransformer{
		transformer: transformer,
	}, nil
}

// Transform 转换文档
func (d *StandardDocumentTransformer) Transform(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	return d.transformer.Transform(ctx, docs)
}
