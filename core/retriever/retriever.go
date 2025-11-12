package retriever

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	milvus "github.com/Malowking/kbgo/milvus_new_re"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
)

// newRetrieverMilvus creates a new Milvus-based retriever
func newRetriever(ctx context.Context, conf *config.Config) (rtr retriever.Retriever, err error) {
	// Get vector field from context, default to FieldContentVector
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

	// Configure Milvus retriever
	retrieverConfig := &milvus.RetrieverConfig{
		Client:            *conf.Client,  // Dereference pointer to get value
		Collection:        conf.Database, // Using Database field as collection name
		VectorField:       vectorField,
		OutputFields:      []string{"id", common.FieldContent, common.FieldExtra, common.KnowledgeId},
		DocumentConverter: MilvusResult2Document,
		MetricType:        milvus.COSINE,
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
func MilvusResult2Document(ctx context.Context, columns []column.Column) ([]*schema.Document, error) {
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
		case common.FieldExtra:
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
					result[i].MetaData[common.FieldExtra] = v
				case []byte:
					result[i].MetaData[common.FieldExtra] = string(v)
				}
			}
		case common.KnowledgeId:
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					continue
				}
				if str, ok := val.(string); ok {
					result[i].MetaData[common.KnowledgeId] = str
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

	return result, nil
}
