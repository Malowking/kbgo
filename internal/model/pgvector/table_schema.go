package pgvector

import (
	"fmt"
)

// TableSchema represents the standard schema for text chunk tables in PostgreSQL with pgvector
// This schema is used for storing document chunks with their embeddings
type TableSchema struct {
	// Id is the unique identifier for each chunk (primary key)
	Id string `pg:"id,varchar(255),primary_key"`

	// Text is the content of the document chunk
	Text string `pg:"text,text"`

	// Vector is the embedding vector of the chunk
	Vector []float32 `pg:"vector,vector"`

	// DocumentId is the ID of the document this chunk belongs to
	DocumentId string `pg:"document_id,varchar(255)"`

	// Metadata stores additional information as JSONB
	Metadata string `pg:"metadata,jsonb"`

	// CreatedAt is the timestamp when the chunk was created
	CreatedAt string `pg:"created_at,timestamp"`
}

// FieldDefinition represents a single field definition in PostgreSQL
type FieldDefinition struct {
	Name        string
	Type        string
	Nullable    bool
	Default     string
	PrimaryKey  bool
	Description string
}

// IndexDefinition represents an index definition in PostgreSQL
type IndexDefinition struct {
	Name        string
	Fields      []string
	IndexType   string // e.g., "btree", "hnsw"
	IndexOps    string // e.g., "vector_cosine_ops", empty for standard btree
	Description string
}

// GetFields returns the PostgreSQL field definitions for text collection
func (TableSchema) GetFields(dim int) []FieldDefinition {
	return []FieldDefinition{
		{
			Name:        "id",
			Type:        "VARCHAR(255)",
			Nullable:    false,
			PrimaryKey:  true,
			Description: "Chunk unique ID (primary key)",
		},
		{
			Name:        "text",
			Type:        "TEXT",
			Nullable:    false,
			Description: "Document chunk content",
		},
		{
			Name:        "vector",
			Type:        fmt.Sprintf("vector(%d)", dim),
			Nullable:    false,
			Description: "Document chunk embedding vector",
		},
		{
			Name:        "document_id",
			Type:        "VARCHAR(255)",
			Nullable:    false,
			Description: "Document ID (foreign key)",
		},
		{
			Name:        "metadata",
			Type:        "JSONB",
			Nullable:    false,
			Default:     "'{}'::jsonb",
			Description: "Additional metadata (JSONB)",
		},
		{
			Name:        "created_at",
			Type:        "TIMESTAMP",
			Nullable:    false,
			Default:     "NOW()",
			Description: "Creation timestamp",
		},
	}
}

// GetIndexes returns the index definitions for the table
func (TableSchema) GetIndexes(tableName string) []IndexDefinition {
	return []IndexDefinition{
		{
			Name:        fmt.Sprintf("%s_vector_idx", tableName),
			Fields:      []string{"vector"},
			IndexType:   "hnsw",
			IndexOps:    "vector_cosine_ops",
			Description: "HNSW index for fast vector similarity search using cosine distance",
		},
		{
			Name:        fmt.Sprintf("%s_document_id_idx", tableName),
			Fields:      []string{"document_id"},
			IndexType:   "btree",
			IndexOps:    "",
			Description: "B-tree index for fast document_id lookups",
		},
	}
}

// GenerateCreateTableSQL generates the CREATE TABLE SQL statement
func (t TableSchema) GenerateCreateTableSQL(schemaName, tableName string, dim int) string {
	fields := t.GetFields(dim)
	fullTableName := fmt.Sprintf("%s.%s", schemaName, tableName)

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", fullTableName)

	// Add field definitions
	for i, field := range fields {
		sql += fmt.Sprintf("    %s %s", field.Name, field.Type)

		if field.PrimaryKey {
			sql += " PRIMARY KEY"
		} else if !field.Nullable {
			sql += " NOT NULL"
		}

		if field.Default != "" && !field.PrimaryKey {
			sql += fmt.Sprintf(" DEFAULT %s", field.Default)
		}

		if i < len(fields)-1 {
			sql += ","
		}
		sql += "\n"
	}

	sql += ")"
	return sql
}

// GenerateCreateIndexSQL generates the CREATE INDEX SQL statements
func (t TableSchema) GenerateCreateIndexSQL(schemaName, tableName string) []string {
	indexes := t.GetIndexes(tableName)
	fullTableName := fmt.Sprintf("%s.%s", schemaName, tableName)

	sqls := make([]string, len(indexes))
	for i, idx := range indexes {
		if idx.IndexType == "hnsw" && idx.IndexOps != "" {
			// HNSW index with custom ops
			sqls[i] = fmt.Sprintf(
				"CREATE INDEX IF NOT EXISTS %s ON %s USING %s (%s %s)",
				idx.Name, fullTableName, idx.IndexType, idx.Fields[0], idx.IndexOps,
			)
		} else {
			// Standard btree index
			sqls[i] = fmt.Sprintf(
				"CREATE INDEX IF NOT EXISTS %s ON %s (%s)",
				idx.Name, fullTableName, idx.Fields[0],
			)
		}
	}

	return sqls
}

// GetStandardTableFields is a helper function to get standard table fields
func GetStandardTableFields(dim int) []FieldDefinition {
	return TableSchema{}.GetFields(dim)
}

// GetStandardTableIndexes is a helper function to get standard table indexes
func GetStandardTableIndexes(tableName string) []IndexDefinition {
	return TableSchema{}.GetIndexes(tableName)
}
