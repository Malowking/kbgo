package milvus

import (
	"github.com/milvus-io/milvus/client/v2/entity"
)

// NL2SQLCollectionSchema represents the schema for NL2SQL metadata collections in Milvus
// This schema is used for storing database schema metadata (tables, columns, metrics, relations) with their embeddings
type NL2SQLCollectionSchema struct {
	// Id is the unique identifier for each entity (primary key)
	Id string `milvus:"id,varchar,256,primary_key"`

	// EntityType is the type of entity: 'table', 'column', 'metric', 'relation'
	EntityType string `milvus:"entity_type,varchar,50"`

	// EntityId is the UUID of the entity in the database
	EntityId string `milvus:"entity_id,varchar,256"`

	// DatasourceId is the ID of the datasource this entity belongs to
	DatasourceId string `milvus:"datasource_id,varchar,256"`

	// Text is the content used for embedding (e.g., table description, column info)
	Text string `milvus:"text,varchar,65535"`

	// Vector is the embedding vector of the entity
	Vector []float32 `milvus:"vector,float_vector,1024"`

	// Metadata stores additional information as JSON (e.g., table_name, column_name, data_type)
	Metadata string `milvus:"metadata,json"`
}

// GetFields returns the Milvus field definitions for NL2SQL collection
func (NL2SQLCollectionSchema) GetFields(dim string) []*entity.Field {
	return []*entity.Field{
		{
			Name:        "id",
			DataType:    entity.FieldTypeVarChar,
			TypeParams:  map[string]string{"max_length": "256"},
			PrimaryKey:  true,
			AutoID:      false,
			Description: "Entity unique ID (primary key)",
		},
		{
			Name:        "entity_type",
			DataType:    entity.FieldTypeVarChar,
			TypeParams:  map[string]string{"max_length": "50"},
			Description: "Entity type: table, column, metric, relation",
		},
		{
			Name:        "entity_id",
			DataType:    entity.FieldTypeVarChar,
			TypeParams:  map[string]string{"max_length": "256"},
			Description: "Entity UUID in database",
		},
		{
			Name:        "datasource_id",
			DataType:    entity.FieldTypeVarChar,
			TypeParams:  map[string]string{"max_length": "256"},
			Description: "Datasource ID (for filtering)",
		},
		{
			Name:        "text",
			DataType:    entity.FieldTypeVarChar,
			TypeParams:  map[string]string{"max_length": "65535"},
			Description: "Text content for embedding",
		},
		{
			Name:        "vector",
			DataType:    entity.FieldTypeFloatVector,
			TypeParams:  map[string]string{"dim": dim},
			Description: "Entity embedding vector",
		},
		{
			Name:        "metadata",
			DataType:    entity.FieldTypeJSON,
			Description: "Additional metadata (JSON)",
		},
	}
}

// GetNL2SQLCollectionFields is a helper function to get NL2SQL collection fields
func GetNL2SQLCollectionFields(dim string) []*entity.Field {
	return NL2SQLCollectionSchema{}.GetFields(dim)
}
