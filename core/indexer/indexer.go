package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	milvusModel "github.com/Malowking/kbgo/internal/model/milvus"
	"github.com/Malowking/kbgo/milvus_new"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus/client/v2/column"
)

// newIndexer creates a new Milvus indexer for the specified collection
func newIndexer(ctx context.Context, conf *config.Config, collectionName string) (idr indexer.Indexer, err error) {
	// Validate configuration
	if collectionName == "" {
		return nil, fmt.Errorf("collection name is required")
	}

	// Create embedding instance
	embeddingIns, err := common.NewEmbedding(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding instance: %w", err)
	}

	// Get appropriate fields for the collection type
	// Set fields and converter based on collection name
	fields := milvusModel.GetStandardCollectionFields()
	documentConverter := getDocumentConverter()

	// Build Milvus Indexer configuration
	indexerConfig := &milvus_new.IndexerConfig{
		Client:            *conf.Client,
		Collection:        collectionName,
		Embedding:         embeddingIns,
		Fields:            fields,
		DocumentConverter: documentConverter,
		MetricType:        milvus_new.L2, // Using L2 distance metric
	}

	// Create Indexer
	idr, err = milvus_new.NewIndexer(ctx, indexerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus indexer for collection '%s': %w",
			collectionName, err)
	}
	g.Log().Infof(ctx, "✅ Successfully created Milvus indexer: collection='%s'", collectionName)
	return idr, nil
}

// NewIndexer 导出的函数，用于创建Milvus索引器
func NewIndexer(ctx context.Context, conf *config.Config, collectionName string) (indexer.Indexer, error) {
	return newIndexer(ctx, conf, collectionName)
}

// getTextDocumentConverter returns the document converter for text collections
func getDocumentConverter() func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
	return func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
		ids := make([]string, len(docs))
		texts := make([]string, len(docs))
		vectorsFloat32 := make([][]float32, len(docs))
		documentIds := make([]string, len(docs))
		metadataList := make([][]byte, len(docs))

		// Extract knowledge base name from context
		var knowledgeId string
		if value, ok := ctx.Value(common.KnowledgeId).(string); ok {
			knowledgeId = value
		}

		// Extract document ID from context
		var contextDocumentId string
		if value, ok := ctx.Value(common.DocumentId).(string); ok {
			contextDocumentId = value
		}

		for idx, doc := range docs {
			// Generate chunk ID if not provided
			if len(doc.ID) == 0 {
				doc.ID = uuid.New().String()
			}
			ids[idx] = doc.ID

			// Truncate text if needed
			texts[idx] = truncateString(doc.Content, 65535)

			// Convert vector to float32
			vectorsFloat32[idx] = float64ToFloat32(vectors[idx])

			// Extract document_id from metadata or use context value
			// Priority: metadata > context
			var docID string
			if contextDocumentId != "" {
				docID = contextDocumentId
			} else {
				// document_id is required, return error if not found
				return nil, fmt.Errorf("document_id not found in metadata or context for chunk %s, document_id is required", doc.ID)
			}
			documentIds[idx] = docID

			// Build metadata JSON
			metaCopy := make(map[string]any)
			if doc.MetaData != nil {
				for k, v := range doc.MetaData {
					metaCopy[k] = v
				}
			}

			// Add knowledge base ID to metadata
			if knowledgeId != "" {
				metaCopy[common.KnowledgeId] = knowledgeId
			}

			// Marshal metadata to JSON bytes
			metaBytes, err := marshalMetadata(metaCopy)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal metadata: %w", err)
			}
			metadataList[idx] = metaBytes
		}

		// Create columns matching the new Milvus schema
		return []column.Column{
			column.NewColumnVarChar("id", ids),                          // chunk 唯一 ID（主键）
			column.NewColumnVarChar("text", texts),                      // chunk 内容
			column.NewColumnFloatVector("vector", 1024, vectorsFloat32), // 向量
			column.NewColumnVarChar("document_id", documentIds),         // 所属文档 ID
			column.NewColumnJSONBytes("metadata", metadataList),         // 元数据
		}, nil
	}
}

// Helper functions

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func float64ToFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}

func marshalMetadata(metadata map[string]any) ([]byte, error) {
	if len(metadata) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(metadata)
}
