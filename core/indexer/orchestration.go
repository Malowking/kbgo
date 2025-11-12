package indexer

import (
	"context"

	"github.com/Malowking/kbgo/core/config"
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
		// QA                   = "QA"
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
	// _ = g.AddLambdaNode(QA, compose.InvokableLambda(qa)) // qa 异步 执行

	_ = g.AddDocumentTransformerNode(DocumentTransformer3, documentTransformer2KeyOfDocumentTransformer)
	_ = g.AddEdge(compose.START, DocumentTransformer3)
	_ = g.AddEdge(DocumentTransformer3, DocAddIDAndMerge)
	_ = g.AddEdge(DocAddIDAndMerge, Indexer)
	// _ = g.AddEdge(DocAddIDAndMerge, QA)
	// _ = g.AddEdge(QA, Indexer2)
	_ = g.AddEdge(Indexer, compose.END)
	r, err = g.Compile(ctx, compose.WithGraphName("indexer"))
	if err != nil {
		return nil, err
	}
	return r, err
}
