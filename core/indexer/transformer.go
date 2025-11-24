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

// NewTransformer 创建文档转换器，支持自定义分隔符
// 如果 separator 为空字符串，则使用默认的递归分割器和 Markdown 分割器
// 如果 separator 不为空，则使用指定的分隔符进行分割
func NewTransformer(ctx context.Context, chunkSize, overlapSize int, separator string) (document.Transformer, error) {
	// 如果指定了分隔符，使用自定义分隔符分割
	if separator != "" {
		// 处理转义字符
		processedSeparator := strings.ReplaceAll(separator, "\\n", "\n")
		processedSeparator = strings.ReplaceAll(processedSeparator, "\\t", "\t")

		config := &recursive.Config{
			ChunkSize:   chunkSize,
			OverlapSize: overlapSize,
			Separators:  []string{processedSeparator},
		}

		return recursive.NewSplitter(ctx, config)
	}

	// 使用默认的转换器（支持 Markdown 和递归分割）
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
