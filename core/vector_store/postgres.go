package vector_store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/dao"
	pgvectorModel "github.com/Malowking/kbgo/internal/model/pgvector"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

// PostgresStore PostgreSQL向量数据库实现
type PostgresStore struct {
	pool     *pgxpool.Pool
	database string
	schema   string // 向量数据存储的 schema
}

// InitializePostgresStore 初始化PostgreSQL向量存储
func InitializePostgresStore(ctx context.Context) (VectorStore, error) {
	host := g.Cfg().MustGet(ctx, "postgres.host", "").String()
	port := g.Cfg().MustGet(ctx, "postgres.port", "5432").String()
	user := g.Cfg().MustGet(ctx, "postgres.user", "").String()
	password := g.Cfg().MustGet(ctx, "postgres.password", "").String()
	database := g.Cfg().MustGet(ctx, "postgres.database", "").String()
	sslMode := g.Cfg().MustGet(ctx, "postgres.sslmode", "disable").String()
	timezone := g.Cfg().MustGet(ctx, "postgres.timezone", "UTC").String()

	if host == "" || user == "" || database == "" {
		return nil, errors.New(errors.ErrVectorStoreInit, "postgres configuration is incomplete. Required: host, user, database")
	}

	// 构建连接字符串
	var connStr string
	if password != "" {
		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
			host, port, user, password, database, sslMode, timezone)
	} else {
		connStr = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s TimeZone=%s",
			host, port, user, database, sslMode, timezone)
	}

	g.Log().Infof(ctx, "Connecting to PostgreSQL at: %s:%s, database: %s", host, port, database)

	// 创建连接池
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, errors.Newf(errors.ErrVectorStoreInit, "failed to create postgres connection pool: %v", err)
	}

	// 测试连接
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.Newf(errors.ErrVectorStoreInit, "failed to ping postgres: %v", err)
	}

	// 创建PostgresStore配置
	config := &VectorStoreConfig{
		Type:     VectorStoreTypePostgreSQL,
		Client:   pool,
		Database: database,
	}

	postgresStore, err := NewVectorStore(config)
	if err != nil {
		pool.Close()
		return nil, errors.Newf(errors.ErrVectorStoreInit, "failed to create postgres store: %v", err)
	}

	return postgresStore, nil
}

// NewPostgresStore 创建PostgreSQL向量存储实例
func NewPostgresStore(config *VectorStoreConfig) (VectorStore, error) {
	if config == nil {
		return nil, errors.New(errors.ErrInvalidParameter, "config cannot be nil")
	}

	pool, ok := config.Client.(*pgxpool.Pool)
	if !ok {
		return nil, errors.New(errors.ErrInvalidParameter, "client must be *pgxpool.Pool")
	}

	if config.Database == "" {
		return nil, errors.New(errors.ErrInvalidParameter, "database name cannot be empty")
	}

	return &PostgresStore{
		pool:     pool,
		database: config.Database,
		schema:   "vectors", // 使用独立的 vectors schema
	}, nil
}

// CreateDatabaseIfNotExists 创建数据库
func (p *PostgresStore) CreateDatabaseIfNotExists(ctx context.Context) error {
	// 1. 检查 pgvector 扩展是否已安装
	var extensionExists bool
	err := p.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&extensionExists)
	if err != nil {
		return errors.Newf(errors.ErrDatabaseQuery, "failed to check pgvector extension: %v", err)
	}

	// 只在扩展不存在时尝试创建
	if !extensionExists {
		g.Log().Infof(ctx, "pgvector extension not found, attempting to create...")
		_, err = p.pool.Exec(ctx, "CREATE EXTENSION vector")
		if err != nil {
			return errors.Newf(errors.ErrVectorStoreInit, "failed to create pgvector extension: %v. Please ensure pgvector is installed for your PostgreSQL version", err)
		}
		g.Log().Infof(ctx, "pgvector extension created successfully")
	} else {
		g.Log().Infof(ctx, "pgvector extension already exists")
	}

	// 2. 创建独立的 vectors schema
	_, err = p.pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", p.schema))
	if err != nil {
		return errors.Newf(errors.ErrVectorStoreInit, "failed to create vectors schema: %v", err)
	}

	g.Log().Infof(ctx, "PostgreSQL database '%s' ready with pgvector extension and '%s' schema", p.database, p.schema)
	return nil
}

// CreateCollection 创建集合
func (p *PostgresStore) CreateCollection(ctx context.Context, collectionName string, dimension int) error {
	// 清理表名，防止SQL注入
	tableName := p.sanitizeTableName(collectionName)

	// 判断是否为NL2SQL集合（以 "nl2sql_" 开头）
	isNL2SQL := strings.HasPrefix(collectionName, "nl2sql_")

	if isNL2SQL {
		// 使用NL2SQL专用表结构
		g.Log().Infof(ctx, "Detected NL2SQL collection, using NL2SQLTableSchema for '%s'", collectionName)
		return p.CreateNL2SQLCollection(ctx, collectionName, dimension)
	}

	// 使用标准表结构模型
	schema := pgvectorModel.TableSchema{}

	// 1. 创建表，使用传入的维度参数
	createTableSQL := schema.GenerateCreateTableSQL(p.schema, tableName, dimension)
	_, err := p.pool.Exec(ctx, createTableSQL)
	if err != nil {
		return errors.Newf(errors.ErrVectorStoreInit, "failed to create table %s.%s: %v", p.schema, tableName, err)
	}

	// 2. 创建索引
	createIndexSQLs := schema.GenerateCreateIndexSQL(p.schema, tableName)
	for _, indexSQL := range createIndexSQLs {
		_, err = p.pool.Exec(ctx, indexSQL)
		if err != nil {
			return errors.Newf(errors.ErrVectorStoreInit, "failed to create index on table %s.%s: %v", p.schema, tableName, err)
		}
	}

	g.Log().Infof(ctx, "Table '%s.%s' created with dimension %d and indexes", p.schema, tableName, dimension)
	return nil
}

// CollectionExists 检查集合
func (p *PostgresStore) CollectionExists(ctx context.Context, collectionName string) (bool, error) {
	tableName := p.sanitizeTableName(collectionName)

	var exists bool
	err := p.pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)",
		p.schema, tableName,
	).Scan(&exists)

	if err != nil {
		return false, errors.Newf(errors.ErrDatabaseQuery, "failed to check if table %s.%s exists: %v", p.schema, tableName, err)
	}

	return exists, nil
}

// DeleteCollection 删除集合
func (p *PostgresStore) DeleteCollection(ctx context.Context, collectionName string) error {
	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	_, err := p.pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", fullTableName))
	if err != nil {
		return errors.Newf(errors.ErrVectorDelete, "failed to drop table %s: %v", fullTableName, err)
	}

	g.Log().Infof(ctx, "Table '%s' deleted", fullTableName)
	return nil
}

// InsertVectors 插入向量数据
func (p *PostgresStore) InsertVectors(ctx context.Context, collectionName string, chunks []*schema.Document, vectors [][]float32) ([]string, error) {
	if len(chunks) != len(vectors) {
		return nil, errors.Newf(errors.ErrInvalidParameter, "chunks and vectors length mismatch: %d vs %d", len(chunks), len(vectors))
	}

	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	// 从上下文中提取knowledge_id和document_id
	var knowledgeId string
	if value, ok := ctx.Value(KnowledgeId).(string); ok {
		knowledgeId = value
	}

	var contextDocumentId string
	if value, ok := ctx.Value(DocumentId).(string); ok {
		contextDocumentId = value
	}

	ids := make([]string, len(chunks))

	// 准备批量插入
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (id, text, vector, document_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, fullTableName)

	for idx, chunk := range chunks {
		// 生成chunk ID
		if len(chunk.ID) == 0 {
			chunk.ID = uuid.New().String()
		}
		ids[idx] = chunk.ID

		// 清理文本并截断
		sanitizedText, err := common.CleanString(chunk.Content, common.ProfileDatabase)
		if err != nil {
			// 如果清洗失败，记录警告但不中断
			g.Log().Warningf(ctx, "Failed to clean chunk content in InsertVectors, using original content. chunkID=%s, err=%v",
				chunk.ID, err)
			sanitizedText = chunk.Content
		}
		text := p.truncateString(sanitizedText, 65535)

		// 转换向量为pgvector格式
		pgVector := pgvector.NewVector(vectors[idx])

		// 设置document_id
		var docID string
		if contextDocumentId != "" {
			docID = contextDocumentId
		} else {
			return nil, errors.Newf(errors.ErrInvalidParameter, "document_id not found in context for chunk %s", chunk.ID)
		}

		// 构建metadata
		metaCopy := make(map[string]any)
		if chunk.MetaData != nil {
			for k, v := range chunk.MetaData {
				metaCopy[k] = v
			}
		}

		// 添加knowledge_id到metadata
		if knowledgeId != "" {
			metaCopy[KnowledgeId] = knowledgeId
		}

		// 序列化metadata
		metaBytes, err := json.Marshal(metaCopy)
		if err != nil {
			return nil, errors.Newf(errors.ErrVectorInsert, "failed to marshal metadata: %v", err)
		}

		// 插入数据
		_, err = tx.Exec(ctx, insertSQL, chunk.ID, text, pgVector, docID, metaBytes)
		if err != nil {
			return nil, errors.Newf(errors.ErrVectorInsert, "failed to insert vector for chunk %s: %v", chunk.ID, err)
		}
	}

	// 提交事务
	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Newf(errors.ErrVectorInsert, "failed to commit transaction: %v", err)
	}

	g.Log().Infof(ctx, "Successfully inserted %d vectors into table '%s'", len(chunks), fullTableName)
	return ids, nil
}

// DeleteByDocumentID 根据文档ID删除所有相关chunks
func (p *PostgresStore) DeleteByDocumentID(ctx context.Context, collectionName string, documentID string) error {
	// 验证 documentID 格式
	if !common.ValidateUUID(documentID) {
		return errors.Newf(errors.ErrInvalidParameter, "invalid document ID format: %s (must be valid UUID)", documentID)
	}

	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	g.Log().Infof(ctx, "Deleting all chunks of document %s from table %s", documentID, fullTableName)

	result, err := p.pool.Exec(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE document_id = $1", fullTableName),
		documentID,
	)
	if err != nil {
		return errors.Newf(errors.ErrVectorDelete, "failed to delete document %s: %v", documentID, err)
	}

	rowsAffected := result.RowsAffected()
	g.Log().Infof(ctx, "Delete operation completed for document %s, affected rows: %d", documentID, rowsAffected)

	if rowsAffected == 0 {
		g.Log().Infof(ctx, "Warning: No chunks were deleted for document_id=%s", documentID)
	}

	return nil
}

// DeleteByChunkID 根据chunkID删除单个chunk
func (p *PostgresStore) DeleteByChunkID(ctx context.Context, collectionName string, chunkID string) error {
	// 验证 chunkID 格式
	if !common.ValidateUUID(chunkID) {
		return errors.Newf(errors.ErrInvalidParameter, "invalid chunk ID format: %s (must be valid UUID)", chunkID)
	}

	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	g.Log().Infof(ctx, "Deleting chunk %s from table %s", chunkID, fullTableName)

	result, err := p.pool.Exec(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE id = $1", fullTableName),
		chunkID,
	)
	if err != nil {
		return errors.Newf(errors.ErrVectorDelete, "failed to delete chunk %s: %v", chunkID, err)
	}

	rowsAffected := result.RowsAffected()
	g.Log().Infof(ctx, "Delete operation completed for chunk %s, affected rows: %d", chunkID, rowsAffected)

	if rowsAffected == 0 {
		g.Log().Infof(ctx, "Warning: No chunk was deleted for id=%s", chunkID)
	}

	return nil
}

// GetClient 返回底层PostgreSQL连接池
func (p *PostgresStore) GetClient() interface{} {
	return p.pool
}

// NewRetriever 创建PostgreSQL检索器实例
func (p *PostgresStore) NewRetriever(ctx context.Context, collectionName string) (Retriever, error) {
	if p.pool == nil {
		return nil, errors.New(errors.ErrInvalidParameter, "postgres pool not provided")
	}

	if collectionName == "" {
		return nil, errors.New(errors.ErrInvalidParameter, "collection name cannot be empty")
	}

	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	// 检查表是否存在
	exists, err := p.CollectionExists(ctx, collectionName)
	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to check collection: %v", err)
	}
	if !exists {
		return nil, errors.Newf(errors.ErrVectorStoreNotFound, "table '%s' not found", fullTableName)
	}

	// 从数据库查询知识库的 embedding 模型 ID
	var embeddingModelID string
	err = dao.GetDB().WithContext(ctx).
		Table("knowledge_base").
		Select("embedding_model_id").
		Where("id = ?", collectionName).
		Scan(&embeddingModelID).Error

	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to query embedding model ID for knowledge %s: %v", collectionName, err)
	}

	if embeddingModelID == "" {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "embedding model ID not found for knowledge %s", collectionName)
	}

	// 从 Registry 获取 embedding 模型配置
	embeddingModel := model.Registry.GetEmbeddingModel(embeddingModelID)
	if embeddingModel == nil {
		return nil, errors.Newf(errors.ErrModelNotFound, "embedding model not found: %s", embeddingModelID)
	}

	// 创建并返回检索器，直接传入 embedding 模型配置
	return &postgresRetriever{
		pool:           p.pool,
		tableName:      fullTableName,  // 使用带 schema 的完整表名
		embeddingModel: embeddingModel, // 直接传入 embedding 模型配置
	}, nil
}

// VectorSearchOnly 仅使用向量检索的通用方法
func (p *PostgresStore) VectorSearchOnly(ctx context.Context, conf GeneralRetrieverConfig, query string, knowledgeId string, topK int, score float64) ([]*schema.Document, error) {
	tableName := p.sanitizeTableName(knowledgeId)

	// 创建检索器
	r, err := p.NewRetriever(ctx, knowledgeId)
	if err != nil {
		g.Log().Errorf(ctx, "failed to create retriever for table %s, err=%v", tableName, err)
		return nil, err
	}

	// PostgreSQL 检索的 TopK，可以设置得比最终需要的数量大一些
	postgresTopK := topK * 5
	if postgresTopK < 20 {
		postgresTopK = 20
	}

	// 执行检索 - 使用反射调用Retrieve方法或者直接类型断言
	if pgRetriever, ok := r.(*postgresRetriever); ok {
		return pgRetriever.vectorSearchWithThreshold(ctx, query, postgresTopK, score)
	}

	return nil, errors.New(errors.ErrVectorSearch, "failed to cast retriever to postgresRetriever")
}

// Helper functions

func (p *PostgresStore) sanitizeTableName(name string) string {
	// 简单的表名清理：只允许字母、数字和下划线
	var result strings.Builder
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' {
			result.WriteRune(char)
		} else {
			result.WriteRune('_')
		}
	}
	return result.String()
}

func (p *PostgresStore) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func (p *PostgresStore) float64ToFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}

// 实现 Retriever 接口
type postgresRetriever struct {
	pool           *pgxpool.Pool
	tableName      string
	embeddingModel *model.EmbeddingModelConfig // 直接存储 embedding 模型配置
}

// Retrieve 实现检索功能
func (r *postgresRetriever) Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error) {
	// 默认参数
	topK := 5
	var scoreThreshold *float64

	// 解析选项
	options := GetCommonOptions(&Options{
		TopK:           &topK,
		ScoreThreshold: scoreThreshold,
	}, opts...)

	if options.TopK != nil {
		topK = *options.TopK
	}
	if options.ScoreThreshold != nil {
		scoreThreshold = options.ScoreThreshold
	}

	// 如果没有设置阈值，使用默认值0.0
	threshold := 0.0
	if scoreThreshold != nil {
		threshold = *scoreThreshold
	}

	return r.vectorSearchWithThreshold(ctx, query, topK, threshold)
}

// vectorSearchWithThreshold 带阈值的向量搜索
func (r *postgresRetriever) vectorSearchWithThreshold(ctx context.Context, query string, topK int, threshold float64) ([]*schema.Document, error) {
	// 直接使用存储的 embedding 模型配置
	if r.embeddingModel == nil {
		return nil, errors.New(errors.ErrEmbeddingFailed, "embedding model not configured for this retriever")
	}

	// 创建 embedder
	embedder, err := common.NewEmbedding(ctx, r.embeddingModel)
	if err != nil {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "failed to create embedder: %v", err)
	}

	// 生成查询向量
	vectors, err := embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "embedding has error: %v", err)
	}

	if len(vectors) != 1 {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	// 直接使用float32向量
	queryVector := pgvector.NewVector(vectors[0])

	// 获取距离度量类型，从配置文件读取
	metricType := g.Cfg().MustGet(ctx, "vectorStore.metricType", "COSINE").String()

	// 根据metricType选择pgvector操作符和分数计算方式
	var scoreCalc, orderBy string
	switch strings.ToUpper(metricType) {
	case "COSINE":
		// 余弦距离: 0=相同, 2=相反
		scoreCalc = "1 - (vector <=> $1)" // 转换为相似度: 1=相同, -1=相反
		orderBy = "vector <=> $1"
	case "L2":
		// 欧氏距离: 0=相同, 越大越远
		scoreCalc = "1 / (1 + (vector <-> $1))" // 归一化: 1=相同, 接近0=很远
		orderBy = "vector <-> $1"
	case "IP", "INNER_PRODUCT":
		// 内积: 值越大越相似
		scoreCalc = "(vector <#> $1)"  // 直接使用内积值作为分数
		orderBy = "vector <#> $1 DESC" // 内积需要降序排列（越大越好）
	default:
		// 默认使用余弦距离
		g.Log().Warningf(ctx, "Unknown metricType '%s', using COSINE as default", metricType)
		scoreCalc = "1 - (vector <=> $1)"
		orderBy = "vector <=> $1"
	}

	// 执行向量相似度搜索
	var searchSQL string
	if threshold > 0 {
		// 有阈值限制
		searchSQL = fmt.Sprintf(`
			SELECT id, text, document_id, metadata,
			       %s as similarity_score
			FROM %s
			WHERE %s >= $2
			ORDER BY %s
			LIMIT $3
		`, scoreCalc, r.tableName, scoreCalc, orderBy)
	} else {
		// 无阈值限制，返回所有结果
		searchSQL = fmt.Sprintf(`
			SELECT id, text, document_id, metadata,
			       %s as similarity_score
			FROM %s
			ORDER BY %s
			LIMIT $2
		`, scoreCalc, r.tableName, orderBy)
	}

	var rows pgx.Rows
	if threshold > 0 {
		rows, err = r.pool.Query(ctx, searchSQL, queryVector, threshold, topK)
	} else {
		rows, err = r.pool.Query(ctx, searchSQL, queryVector, topK)
	}
	if err != nil {
		return nil, errors.Newf(errors.ErrVectorSearch, "failed to execute vector search: %v", err)
	}
	defer rows.Close()

	var results []*schema.Document
	for rows.Next() {
		var id, text, documentId string
		var metadataBytes []byte
		var score float64

		err := rows.Scan(&id, &text, &documentId, &metadataBytes, &score)
		if err != nil {
			return nil, errors.Newf(errors.ErrVectorSearch, "failed to scan row: %v", err)
		}

		doc := &schema.Document{
			ID:       id,
			Content:  text,
			MetaData: make(map[string]any),
		}
		doc.Score = float32(score)

		// 解析metadata
		if len(metadataBytes) > 0 {
			var metadata map[string]any
			if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
				for k, v := range metadata {
					doc.MetaData[k] = v
				}
			}
		}

		// 添加document_id到metadata
		doc.MetaData[DocumentId] = documentId

		results = append(results, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Newf(errors.ErrVectorSearch, "error iterating over rows: %v", err)
	}

	g.Log().Infof(ctx, "Vector search SQL returned %d rows", len(results))

	// 权限控制：过滤掉status != 1的chunks
	if len(results) > 0 {
		chunkIDs := make([]string, 0, len(results))
		for _, doc := range results {
			chunkIDs = append(chunkIDs, doc.ID)
		}

		activeIDs, err := dao.GetActiveChunkIDs(ctx, chunkIDs)
		if err != nil {
			return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to query chunk status: %v", err)
		}

		g.Log().Infof(ctx, "Permission filter: %d active chunks out of %d total", len(activeIDs), len(chunkIDs))

		// 收集document_ids以查询文档名称
		documentIDsMap := make(map[string]bool)
		for _, doc := range results {
			if docID, ok := doc.MetaData[DocumentId].(string); ok && docID != "" {
				documentIDsMap[docID] = true
			}
		}

		// 查询文档名称
		docNameMap := make(map[string]string)
		if len(documentIDsMap) > 0 {
			documentIDs := make([]string, 0, len(documentIDsMap))
			for docID := range documentIDsMap {
				documentIDs = append(documentIDs, docID)
			}

			// 使用 gorm 查询文档名称
			var documents []struct {
				ID       string
				FileName string `gorm:"column:file_name"`
			}
			err := dao.GetDB().WithContext(ctx).
				Table("knowledge_documents").
				Select("id, file_name").
				Where("id IN ?", documentIDs).
				Find(&documents).Error

			if err == nil {
				for _, doc := range documents {
					docNameMap[doc.ID] = doc.FileName
				}
			}
		}

		// 过滤结果并添加文档名称
		filtered := make([]*schema.Document, 0, len(results))
		for _, doc := range results {
			if activeIDs[doc.ID] {
				// 添加文档名称到metadata
				if docID, ok := doc.MetaData[DocumentId].(string); ok {
					if docName, exists := docNameMap[docID]; exists {
						doc.MetaData["document_name"] = docName
					}
				}
				filtered = append(filtered, doc)
			}
		}

		results = filtered
	}

	// 去重
	results = common.RemoveDuplicates(results, func(doc *schema.Document) string {
		return doc.ID
	})

	// 按相似度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func (r *postgresRetriever) float64ToFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}

// GetType 返回检索器类型
func (r *postgresRetriever) GetType() string {
	return "PostgresRetriever"
}

// IsCallbacksEnabled 返回是否启用回调
func (r *postgresRetriever) IsCallbacksEnabled() bool {
	return false
}

// ==================== NL2SQL专用方法 ====================

// CreateNL2SQLCollection 创建NL2SQL专用的集合
func (p *PostgresStore) CreateNL2SQLCollection(ctx context.Context, collectionName string, dimension int) error {
	// 清理表名，防止SQL注入
	tableName := p.sanitizeTableName(collectionName)

	// 使用NL2SQL专用表结构模型
	schema := pgvectorModel.NL2SQLTableSchema{}

	// 1. 创建表，使用传入的维度参数
	createTableSQL := schema.GenerateCreateTableSQL(p.schema, tableName, dimension)
	_, err := p.pool.Exec(ctx, createTableSQL)
	if err != nil {
		return errors.Newf(errors.ErrVectorStoreInit, "failed to create NL2SQL table %s.%s: %v", p.schema, tableName, err)
	}

	// 2. 创建索引
	createIndexSQLs := schema.GenerateCreateIndexSQL(p.schema, tableName)
	for _, indexSQL := range createIndexSQLs {
		_, err := p.pool.Exec(ctx, indexSQL)
		if err != nil {
			g.Log().Warningf(ctx, "failed to create index for NL2SQL table %s.%s: %v", p.schema, tableName, err)
			// 继续执行，索引创建失败不应该阻止表的使用
		}
	}

	g.Log().Infof(ctx, "NL2SQL table '%s.%s' created with dimension %d and indexes", p.schema, tableName, dimension)
	return nil
}

// InsertNL2SQLVectors 插入NL2SQL向量数据
func (p *PostgresStore) InsertNL2SQLVectors(ctx context.Context, collectionName string, entities []*NL2SQLEntity, vectors [][]float32) ([]string, error) {
	if len(entities) == 0 {
		return []string{}, nil
	}

	if len(entities) != len(vectors) {
		return nil, errors.Newf(errors.ErrInvalidParameter, "entities count (%d) must match vectors count (%d)", len(entities), len(vectors))
	}

	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	// 构建批量插入SQL
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (id, entity_type, entity_id, datasource_id, text, vector, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, fullTableName)

	// 使用事务批量插入
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, errors.Newf(errors.ErrVectorInsert, "failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	ids := make([]string, len(entities))
	for i, entity := range entities {
		// 生成ID（如果没有）
		if entity.ID == "" {
			entity.ID = uuid.New().String()
		}
		ids[i] = entity.ID

		// 序列化metadata
		metadataJSON, err := json.Marshal(entity.MetaData)
		if err != nil {
			return nil, errors.Newf(errors.ErrInvalidParameter, "failed to marshal metadata for entity %s: %v", entity.ID, err)
		}

		// 转换向量为pgvector格式
		vec := pgvector.NewVector(vectors[i])

		// 执行插入
		_, err = tx.Exec(ctx, insertSQL,
			entity.ID,
			entity.EntityType,
			entity.EntityID,
			entity.DatasourceID,
			entity.Text,
			vec,
			metadataJSON,
		)
		if err != nil {
			return nil, errors.Newf(errors.ErrVectorInsert, "failed to insert NL2SQL entity %s: %v", entity.ID, err)
		}
	}

	// 提交事务
	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Newf(errors.ErrVectorInsert, "failed to commit transaction: %v", err)
	}

	g.Log().Infof(ctx, "Inserted %d NL2SQL entities into table '%s'", len(entities), fullTableName)
	return ids, nil
}

// DeleteNL2SQLByDatasourceID 根据数据源ID删除所有相关实体
func (p *PostgresStore) DeleteNL2SQLByDatasourceID(ctx context.Context, collectionName string, datasourceID string) error {
	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	// 构建删除SQL
	deleteSQL := fmt.Sprintf(`DELETE FROM %s WHERE datasource_id = $1`, fullTableName)

	// 执行删除
	result, err := p.pool.Exec(ctx, deleteSQL, datasourceID)
	if err != nil {
		return errors.Newf(errors.ErrVectorDelete, "failed to delete NL2SQL entities by datasource_id: %v", err)
	}

	rowsAffected := result.RowsAffected()
	g.Log().Infof(ctx, "Deleted %d NL2SQL entities with datasource_id '%s' from table '%s'", rowsAffected, datasourceID, fullTableName)
	return nil
}

// NewNL2SQLRetriever 创建NL2SQL专用的PostgreSQL检索器实例
func (p *PostgresStore) NewNL2SQLRetriever(ctx context.Context, collectionName string, datasourceID string) (Retriever, error) {
	if p.pool == nil {
		return nil, errors.New(errors.ErrInvalidParameter, "postgres pool not provided")
	}

	if collectionName == "" {
		return nil, errors.New(errors.ErrInvalidParameter, "collection name cannot be empty")
	}

	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	// 检查表是否存在
	exists, err := p.CollectionExists(ctx, collectionName)
	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to check collection: %v", err)
	}
	if !exists {
		return nil, errors.Newf(errors.ErrVectorStoreNotFound, "table '%s' not found", fullTableName)
	}

	// 获取 embedding 模型 ID
	var embeddingModelID string
	err = dao.GetDB().WithContext(ctx).
		Table("nl2sql_datasources").
		Select("embedding_model_id").
		Where("id = ?", datasourceID).
		Scan(&embeddingModelID).Error

	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to query embedding model ID for datasources %s: %v", datasourceID, err)
	}

	if embeddingModelID == "" {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "embedding model ID not found for datasources %s", datasourceID)
	}

	// 从 Registry 获取 embedding 模型配置
	embeddingModel := model.Registry.GetEmbeddingModel(embeddingModelID)
	if embeddingModel == nil {
		return nil, errors.Newf(errors.ErrModelNotFound, "embedding model not found: %s", embeddingModelID)
	}

	// 创建并返回NL2SQL检索器，直接传入 embedding 模型配置
	return &nl2sqlRetriever{
		pool:           p.pool,
		tableName:      fullTableName,
		datasourceID:   datasourceID,
		embeddingModel: embeddingModel, // 直接传入 embedding 模型配置
	}, nil
}

// nl2sqlRetriever NL2SQL专用检索器
type nl2sqlRetriever struct {
	pool           *pgxpool.Pool
	tableName      string
	datasourceID   string
	embeddingModel *model.EmbeddingModelConfig // 直接存储 embedding 模型配置
}

// Retrieve 实现NL2SQL检索功能
func (r *nl2sqlRetriever) Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error) {
	// 默认参数
	topK := 5
	var scoreThreshold *float64

	// 解析选项
	options := GetCommonOptions(&Options{
		TopK:           &topK,
		ScoreThreshold: scoreThreshold,
	}, opts...)

	if options.TopK != nil {
		topK = *options.TopK
	}
	if options.ScoreThreshold != nil {
		scoreThreshold = options.ScoreThreshold
	}

	// 如果没有设置阈值，使用默认值0.0
	threshold := 0.0
	if scoreThreshold != nil {
		threshold = *scoreThreshold
	}

	return r.nl2sqlVectorSearchWithThreshold(ctx, query, topK, threshold)
}

// nl2sqlVectorSearchWithThreshold NL2SQL专用的向量搜索
func (r *nl2sqlRetriever) nl2sqlVectorSearchWithThreshold(ctx context.Context, query string, topK int, threshold float64) ([]*schema.Document, error) {
	// 直接使用存储的 embedding 模型配置
	if r.embeddingModel == nil {
		return nil, errors.New(errors.ErrEmbeddingFailed, "embedding model not configured for this retriever")
	}

	// 创建 embedder
	embedder, err := common.NewEmbedding(ctx, r.embeddingModel)
	if err != nil {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "failed to create embedder: %v", err)
	}

	// 生成查询向量
	vectors, err := embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "embedding has error: %v", err)
	}

	if len(vectors) != 1 {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	queryVector := pgvector.NewVector(vectors[0])

	// 获取距离度量类型
	metricType := g.Cfg().MustGet(ctx, "vectorStore.metricType", "COSINE").String()

	// 根据metricType选择pgvector操作符和分数计算方式
	var scoreCalc, orderBy string
	switch strings.ToUpper(metricType) {
	case "COSINE":
		scoreCalc = "1 - (vector <=> $1)"
		orderBy = "vector <=> $1"
	case "L2":
		scoreCalc = "1 / (1 + (vector <-> $1))"
		orderBy = "vector <-> $1"
	case "IP", "INNER_PRODUCT":
		scoreCalc = "(vector <#> $1)"
		orderBy = "vector <#> $1 DESC"
	default:
		g.Log().Warningf(ctx, "Unknown metricType '%s', using COSINE as default", metricType)
		scoreCalc = "1 - (vector <=> $1)"
		orderBy = "vector <=> $1"
	}

	// 执行NL2SQL向量相似度搜索（包含datasource_id过滤）
	searchSQL := fmt.Sprintf(`
		SELECT id, entity_type, entity_id, datasource_id, text, metadata,
		       %s as similarity_score
		FROM %s
		WHERE datasource_id = $2 AND %s >= $3
		ORDER BY %s
		LIMIT $4
	`, scoreCalc, r.tableName, scoreCalc, orderBy)

	rows, err := r.pool.Query(ctx, searchSQL, queryVector, r.datasourceID, threshold, topK)
	if err != nil {
		return nil, errors.Newf(errors.ErrVectorSearch, "failed to execute NL2SQL vector search: %v", err)
	}
	defer rows.Close()

	var results []*schema.Document
	for rows.Next() {
		var id, entityType, entityID, datasourceID, text string
		var metadataBytes []byte
		var score float64

		err := rows.Scan(&id, &entityType, &entityID, &datasourceID, &text, &metadataBytes, &score)
		if err != nil {
			return nil, errors.Newf(errors.ErrVectorSearch, "failed to scan row: %v", err)
		}

		doc := &schema.Document{
			ID:       id,
			Content:  text,
			MetaData: make(map[string]any),
		}
		doc.Score = float32(score)

		// 解析metadata
		if len(metadataBytes) > 0 {
			var metadata map[string]any
			if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
				for k, v := range metadata {
					doc.MetaData[k] = v
				}
			}
		}

		// 添加NL2SQL特定字段到metadata
		doc.MetaData[NL2SQLFieldEntityType] = entityType
		doc.MetaData[NL2SQLFieldEntityId] = entityID
		doc.MetaData[NL2SQLFieldDatasourceId] = datasourceID

		results = append(results, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Newf(errors.ErrVectorSearch, "error iterating over rows: %v", err)
	}

	// 去重
	results = common.RemoveDuplicates(results, func(doc *schema.Document) string {
		return doc.ID
	})

	// 按相似度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	g.Log().Infof(ctx, "NL2SQL vector search completed: found %d results for datasource %s", len(results), r.datasourceID)
	return results, nil
}

// GetType 返回检索器类型
func (r *nl2sqlRetriever) GetType() string {
	return "NL2SQLRetriever"
}

// IsCallbacksEnabled 返回是否启用回调
func (r *nl2sqlRetriever) IsCallbacksEnabled() bool {
	return false
}

// VectorSearchOnlyNL2SQL NL2SQL专用的向量检索方法
func (p *PostgresStore) VectorSearchOnlyNL2SQL(ctx context.Context, query string, collectionName string, datasourceID string, topK int, score float64) ([]*schema.Document, error) {
	// 创建NL2SQL检索器
	r, err := p.NewNL2SQLRetriever(ctx, collectionName, datasourceID)
	if err != nil {
		g.Log().Errorf(ctx, "failed to create NL2SQL retriever for collection %s, err=%v", collectionName, err)
		return nil, err
	}

	// 执行检索
	if nl2sqlRetriever, ok := r.(*nl2sqlRetriever); ok {
		return nl2sqlRetriever.nl2sqlVectorSearchWithThreshold(ctx, query, topK, score)
	}

	return nil, errors.New(errors.ErrVectorSearch, "failed to cast retriever to nl2sqlRetriever")
}
