package milvus

import (
	"github.com/milvus-io/milvus/client/v2/entity"
)

// TextCollectionSchema represents the standard schema for text chunk collections in Milvus
// This schema is used for storing document chunks with their embeddings
type CollectionSchema struct {
	// Id is the unique identifier for each chunk (primary key)
	Id string `milvus:"id,varchar,256,primary_key"`

	// Text is the content of the document chunk
	Text string `milvus:"text,varchar,65535"`

	// Vector is the embedding vector of the chunk
	Vector []float32 `milvus:"vector,float_vector,1024"`

	// DocumentId is the ID of the document this chunk belongs to
	DocumentId string `milvus:"document_id,varchar,256"`

	// Metadata stores additional information as JSON
	Metadata string `milvus:"metadata,json"`
}

// GetFields returns the Milvus field definitions for text collection
func (CollectionSchema) GetFields() []*entity.Field {
	return []*entity.Field{
		{
			Name:        "id",
			DataType:    entity.FieldTypeVarChar,
			TypeParams:  map[string]string{"max_length": "256"},
			PrimaryKey:  true,
			AutoID:      false,
			Description: "Chunk unique ID (primary key)",
		},
		{
			Name:        "text",
			DataType:    entity.FieldTypeVarChar,
			TypeParams:  map[string]string{"max_length": "65535"},
			Description: "Document chunk content",
		},
		{
			Name:        "vector",
			DataType:    entity.FieldTypeFloatVector,
			TypeParams:  map[string]string{"dim": "1024"},
			Description: "Document chunk embedding vector",
		},
		{
			Name:        "document_id",
			DataType:    entity.FieldTypeVarChar,
			TypeParams:  map[string]string{"max_length": "256"},
			Description: "Document ID (foreign key)",
		},
		{
			Name:        "metadata",
			DataType:    entity.FieldTypeJSON,
			Description: "Additional metadata (JSON)",
		},
	}
}

// GetStandardTextCollectionFields is a helper function to get standard text collection fields
func GetStandardCollectionFields() []*entity.Field {
	return CollectionSchema{}.GetFields()
}
