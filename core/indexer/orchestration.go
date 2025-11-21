package indexer

import (
	"context"

	"github.com/Malowking/kbgo/core/config"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino/compose"
)

// BuildIndexer builds an indexer for the specified collection
// collectionName: the Milvus collection name
// NOTE: This function expects pre-processed documents as input (not file paths/URLs)
func BuildIndexer(ctx context.Context, conf *config.Config, collectionName string, chunkSize, overlapSize int) (r compose.Runnable[any, []string], err error) {
	const (
		Indexer              = "Indexer"
		DocumentTransformer3 = "DocumentTransformer"
		DocAddIDAndMerge     = "DocAddIDAndMerge"
	)

	g := compose.NewGraph[any, []string]()
	indexer2KeyOfIndexer, err := newIndexer(ctx, conf, collectionName)
	if err != nil {
		return nil, err
	}
	_ = g.AddIndexerNode(Indexer, indexer2KeyOfIndexer)
	documentTransformer2KeyOfDocumentTransformer, err := newDocumentTransformer(ctx, chunkSize, overlapSize)
	if err != nil {
		return nil, err
	}
	_ = g.AddLambdaNode(DocAddIDAndMerge, compose.InvokableLambda(docAddIDAndMerge))

	_ = g.AddDocumentTransformerNode(DocumentTransformer3, documentTransformer2KeyOfDocumentTransformer)
	_ = g.AddEdge(compose.START, DocumentTransformer3)
	_ = g.AddEdge(DocumentTransformer3, DocAddIDAndMerge)
	_ = g.AddEdge(DocAddIDAndMerge, Indexer)
	_ = g.AddEdge(Indexer, compose.END)
	r, err = g.Compile(ctx, compose.WithGraphName("indexer"))
	if err != nil {
		return nil, err
	}
	return r, err
}

// BuildIndexerWithSeparator builds an indexer for the specified collection with custom separator
// collectionName: the Milvus collection name
// separator: custom separator for document splitting
func BuildIndexerWithSeparator(ctx context.Context, conf *config.Config, collectionName string, chunkSize, overlapSize int, separator string) (r compose.Runnable[any, []string], err error) {
	const (
		Indexer              = "Indexer"
		DocumentTransformer3 = "DocumentTransformer"
		DocAddIDAndMerge     = "DocAddIDAndMerge"
	)

	g := compose.NewGraph[any, []string]()
	indexer2KeyOfIndexer, err := newIndexer(ctx, conf, collectionName)
	if err != nil {
		return nil, err
	}
	_ = g.AddIndexerNode(Indexer, indexer2KeyOfIndexer)

	// 使用自定义分隔符创建文档转换器
	documentTransformer2KeyOfDocumentTransformer, err := SeparatorSplitter(ctx, separator, chunkSize, overlapSize)
	if err != nil {
		return nil, err
	}

	// 创建一个包含markdown处理器和自定义分隔符处理器的转换器
	trans := &transformer{
		recursive: documentTransformer2KeyOfDocumentTransformer,
		separator: separator,
	}

	// md 文档特殊处理
	mdTrans, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers:     map[string]string{"#": "h1", "##": "h2", "###": "h3"},
		TrimHeaders: false,
	})
	if err != nil {
		return nil, err
	}
	trans.markdown = mdTrans

	_ = g.AddLambdaNode(DocAddIDAndMerge, compose.InvokableLambda(docAddIDAndMerge))

	_ = g.AddDocumentTransformerNode(DocumentTransformer3, trans)
	_ = g.AddEdge(compose.START, DocumentTransformer3)
	_ = g.AddEdge(DocumentTransformer3, DocAddIDAndMerge)
	_ = g.AddEdge(DocAddIDAndMerge, Indexer)
	_ = g.AddEdge(Indexer, compose.END)
	r, err = g.Compile(ctx, compose.WithGraphName("indexer"))
	if err != nil {
		return nil, err
	}
	return r, err
}
