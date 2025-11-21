package indexer

import (
	"context"
	"strings"

	"github.com/Malowking/kbgo/core/common"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// newDocumentTransformer component initialization function of node 'DocumentTransformer3' in graph 'rag'
func newDocumentTransformer(ctx context.Context, chunkSize, overlapSize int) (tfr document.Transformer, err error) {
	trans := &transformer{}
	// 递归分割
	config := &recursive.Config{
		ChunkSize:   chunkSize,   // 自定义每段内容大小
		OverlapSize: overlapSize, // 自定义重叠大小
		Separators:  []string{"\n", "。", "?", "？", "!", "！"},
	}
	recTrans, err := recursive.NewSplitter(ctx, config)
	if err != nil {
		return nil, err
	}
	// md 文档特殊处理
	mdTrans, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers:     map[string]string{"#": common.Title1, "##": common.Title2, "###": common.Title3},
		TrimHeaders: false,
	})
	if err != nil {
		return nil, err
	}
	trans.recursive = recTrans
	trans.markdown = mdTrans
	return trans, nil
}

// NewTransformer 导出的函数，用于创建文档转换器
func NewTransformer(ctx context.Context, chunkSize, overlapSize int) (document.Transformer, error) {
	return newDocumentTransformer(ctx, chunkSize, overlapSize)
}

// NewTransformerWithSeparator 导出的函数，用于创建基于指定分隔符的文档转换器
func NewTransformerWithSeparator(ctx context.Context, chunkSize, overlapSize int, separator string) (document.Transformer, error) {
	return SeparatorSplitter(ctx, separator, chunkSize, overlapSize)
}

type transformer struct {
	markdown  document.Transformer
	recursive document.Transformer
	separator string // 自定义分隔符
}

func (x *transformer) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	isMd := false
	for _, doc := range docs {
		// 只需要判断第一个是不是.md
		if doc.MetaData["_extension"] == ".md" {
			isMd = true
			break
		}
	}
	if isMd {
		return x.markdown.Transform(ctx, docs, opts...)
	}
	return x.recursive.Transform(ctx, docs, opts...)
}

// SeparatorSplitter 基于指定分隔符的文档分割器
func SeparatorSplitter(ctx context.Context, separator string, chunkSize, overlapSize int) (document.Transformer, error) {
	// 处理转义字符
	processedSeparator := strings.ReplaceAll(separator, "\\n", "\n")
	processedSeparator = strings.ReplaceAll(separator, "\\t", "\t")

	config := &recursive.Config{
		ChunkSize:   chunkSize,
		OverlapSize: overlapSize,
		Separators:  []string{processedSeparator},
	}

	return recursive.NewSplitter(ctx, config)
}
