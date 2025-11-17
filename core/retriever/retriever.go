package retriever

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/internal/dao"
	milvus "github.com/Malowking/kbgo/milvus_new_re"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
)

func Retriever(ctx context.Context, conf *config.Config, collectionName string) (rtr retriever.Retriever, err error) {
	vectorField := common.FieldContentVector
	if value, ok := ctx.Value(common.RetrieverFieldKey).(string); ok {
		vectorField = value
	}

	// Create embedding instance
	embeddingIns, err := common.NewEmbedding(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding: %w", err)
	}

	// Check if Client is nil
	if conf.Client == nil {
		return nil, fmt.Errorf("milvus client is nil")
	}

	// 设置 MetricType，如果配置文件中有指定则使用，否则默认使用 COSINE
	metricType := milvus.COSINE
	if conf.MetricType != "" {
		metricType = milvus.MetricType(conf.MetricType)
	}

	// Configure Milvus retriever
	retrieverConfig := &milvus.RetrieverConfig{
		Client:            *conf.Client,
		Collection:        collectionName,
		VectorField:       vectorField,
		OutputFields:      []string{"id", common.FieldContent, common.FieldMetadata, common.DocumentId},
		DocumentConverter: MilvusResult2Document,
		MetricType:        metricType,
		TopK:              5,
		ScoreThreshold:    0,
		Embedding:         embeddingIns,
	}

	// Create Milvus retriever
	rtr, err = milvus.NewRetriever(ctx, retrieverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus retriever: %w", err)
	}

	return rtr, nil
}

// MilvusResult2Document converts Milvus search results to schema.Document
// and filters out chunks with status != 1 for permission control
func MilvusResult2Document(ctx context.Context, columns []column.Column, scores []float32) ([]*schema.Document, error) {
	if len(columns) == 0 {
		return nil, nil
	}

	// Determine the number of documents from the first column
	numDocs := columns[0].Len()
	result := make([]*schema.Document, numDocs)
	for i := range result {
		result[i] = &schema.Document{
			MetaData: make(map[string]any),
		}
	}

	// Set scores for each document
	for i := 0; i < numDocs && i < len(scores); i++ {
		result[i].WithScore(float64(scores[i]))
	}

	// Process each column
	for _, col := range columns {
		switch col.Name() {
		case "id":
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					return nil, fmt.Errorf("failed to get id: %w", err)
				}
				if str, ok := val.(string); ok {
					result[i].ID = str
				}
			}
		case common.FieldContent:
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					return nil, fmt.Errorf("failed to get content: %w", err)
				}
				if str, ok := val.(string); ok {
					result[i].Content = str
				}
			}
		case common.FieldContentVector:
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					return nil, fmt.Errorf("failed to get content_vector: %w", err)
				}
				// Milvus returns vectors as []float32 or []byte, convert to []float64
				switch v := val.(type) {
				case []float32:
					vec := make([]float64, len(v))
					for j, f := range v {
						vec[j] = float64(f)
					}
					result[i].WithDenseVector(vec)
				case []float64:
					result[i].WithDenseVector(v)
				}
			}
		case common.FieldMetadata:
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					continue
				}
				if val == nil {
					continue
				}
				// Handle both string and []byte for extra field
				switch v := val.(type) {
				case string:
					result[i].MetaData[common.FieldMetadata] = v
				case []byte:
					result[i].MetaData[common.FieldMetadata] = string(v)
				}
			}
		case common.DocumentId:
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					continue
				}
				if str, ok := val.(string); ok {
					result[i].MetaData[common.DocumentId] = str
				}
			}
		default:
			// Add other fields to metadata
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					continue
				}
				result[i].MetaData[col.Name()] = val
			}
		}
	}

	// Permission control: Filter out chunks with status != 1
	// Collect all chunk IDs
	chunkIDs := make([]string, 0, len(result))
	for _, doc := range result {
		if doc.ID != "" {
			chunkIDs = append(chunkIDs, doc.ID)
		}
	}

	// Query active chunk IDs from database
	if len(chunkIDs) > 0 {
		activeIDs, err := dao.KnowledgeChunks.GetActiveChunkIDs(ctx, chunkIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to query chunk status: %w", err)
		}

		// Filter documents to only include active chunks
		filtered := make([]*schema.Document, 0, len(result))
		for _, doc := range result {
			if activeIDs.Contains(doc.ID) {
				filtered = append(filtered, doc)
			}
		}

		return filtered, nil
	}

	return result, nil
}
