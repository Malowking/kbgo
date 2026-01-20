package pgvector

import (
	"fmt"
)

// NL2SQLTableSchema represents the schema for NL2SQL metadata tables in PostgreSQL with pgvector
// This schema is used for storing database schema metadata (tables, columns, metrics, relations) with their embeddings
type NL2SQLTableSchema struct {
	// Id is the unique identifier for each entity (primary key)
	Id string `pg:"id,varchar(255),primary_key"`

	// EntityType is the type of entity: 'table', 'column', 'metric', 'relation'
	EntityType string `pg:"entity_type,varchar(50)"`

	// EntityId is the UUID of the entity in the database
	EntityId string `pg:"entity_id,varchar(255)"`

	// DatasourceId is the ID of the datasource this entity belongs to
	DatasourceId string `pg:"datasource_id,varchar(255)"`

	// Text is the content used for embedding (e.g., table description, column info)
	Text string `pg:"text,text"`

	// Vector is the embedding vector of the entity
	Vector []float32 `pg:"vector,vector"`

	// Metadata stores additional information as JSONB (e.g., table_name, column_name, data_type)
	Metadata string `pg:"metadata,jsonb"`

	// CreatedAt is the timestamp when the entity was created
	CreatedAt string `pg:"created_at,timestamp"`
}

// GetFields returns the PostgreSQL field definitions for NL2SQL table
func (NL2SQLTableSchema) GetFields(dim int) []FieldDefinition {
	return []FieldDefinition{
		{
			Name:        "id",
			Type:        "VARCHAR(255)",
			Nullable:    false,
			PrimaryKey:  true,
			Description: "Entity unique ID (primary key)",
		},
		{
			Name:        "entity_type",
			Type:        "VARCHAR(50)",
			Nullable:    false,
			Description: "Entity type: table, column, metric, relation",
		},
		{
			Name:        "entity_id",
			Type:        "VARCHAR(255)",
			Nullable:    false,
			Description: "Entity UUID in database",
		},
		{
			Name:        "datasource_id",
			Type:        "VARCHAR(255)",
			Nullable:    false,
			Description: "Datasource ID (for filtering)",
		},
		{
			Name:        "text",
			Type:        "TEXT",
			Nullable:    false,
			Description: "Text content for embedding",
		},
		{
			Name:        "vector",
			Type:        fmt.Sprintf("vector(%d)", dim),
			Nullable:    false,
			Description: "Entity embedding vector",
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

// GetIndexes returns the index definitions for the NL2SQL table
func (NL2SQLTableSchema) GetIndexes(tableName string) []IndexDefinition {
	return []IndexDefinition{
		{
			Name:        fmt.Sprintf("%s_vector_idx", tableName),
			Fields:      []string{"vector"},
			IndexType:   "hnsw",
			IndexOps:    "vector_cosine_ops",
			Description: "HNSW index for fast vector similarity search using cosine distance",
		},
		{
			Name:        fmt.Sprintf("%s_datasource_id_idx", tableName),
			Fields:      []string{"datasource_id"},
			IndexType:   "btree",
			IndexOps:    "",
			Description: "B-tree index for fast datasource_id lookups",
		},
		{
			Name:        fmt.Sprintf("%s_entity_type_idx", tableName),
			Fields:      []string{"entity_type"},
			IndexType:   "btree",
			IndexOps:    "",
			Description: "B-tree index for fast entity_type filtering",
		},
		{
			Name:        fmt.Sprintf("%s_entity_id_idx", tableName),
			Fields:      []string{"entity_id"},
			IndexType:   "btree",
			IndexOps:    "",
			Description: "B-tree index for fast entity_id lookups",
		},
	}
}

// GenerateCreateTableSQL generates the CREATE TABLE SQL statement for NL2SQL
func (t NL2SQLTableSchema) GenerateCreateTableSQL(schemaName, tableName string, dim int) string {
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

// GenerateCreateIndexSQL generates the CREATE INDEX SQL statements for NL2SQL
func (t NL2SQLTableSchema) GenerateCreateIndexSQL(schemaName, tableName string) []string {
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

// GetNL2SQLTableFields is a helper function to get NL2SQL table fields
func GetNL2SQLTableFields(dim int) []FieldDefinition {
	return NL2SQLTableSchema{}.GetFields(dim)
}

// GetNL2SQLTableIndexes is a helper function to get NL2SQL table indexes
func GetNL2SQLTableIndexes(tableName string) []IndexDefinition {
	return NL2SQLTableSchema{}.GetIndexes(tableName)
}
