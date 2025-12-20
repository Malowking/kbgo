package vector_store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/dao"
	pgvectorModel "github.com/Malowking/kbgo/internal/model/pgvector"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
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

	if host == "" || user == "" || database == "" {
		return nil, fmt.Errorf("postgres configuration is incomplete. Required: host, user, database")
	}

	// 构建连接字符串（去掉空密码的 password= 参数）
	var connStr string
	if password != "" {
		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, database, sslMode)
	} else {
		connStr = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s",
			host, port, user, database, sslMode)
	}

	g.Log().Infof(ctx, "Connecting to PostgreSQL at: %s:%s, database: %s", host, port, database)

	// 创建连接池
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres connection pool: %w", err)
	}

	// 测试连接
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
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
		return nil, fmt.Errorf("failed to create postgres store: %w", err)
	}

	return postgresStore, nil
}

// NewPostgresStore 创建PostgreSQL向量存储实例
func NewPostgresStore(config *VectorStoreConfig) (VectorStore, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	pool, ok := config.Client.(*pgxpool.Pool)
	if !ok {
		return nil, fmt.Errorf("client must be *pgxpool.Pool")
	}

	if config.Database == "" {
		return nil, fmt.Errorf("database name cannot be empty")
	}

	return &PostgresStore{
		pool:     pool,
		database: config.Database,
		schema:   "vectors", // 使用独立的 vectors schema
	}, nil
}

// CreateDatabaseIfNotExists 创建数据库（如果不存在）- PostgreSQL版本
func (p *PostgresStore) CreateDatabaseIfNotExists(ctx context.Context) error {
	// 1. 检查 pgvector 扩展是否已安装
	var extensionExists bool
	err := p.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&extensionExists)
	if err != nil {
		return fmt.Errorf("failed to check pgvector extension: %w", err)
	}

	// 只在扩展不存在时尝试创建
	if !extensionExists {
		g.Log().Infof(ctx, "pgvector extension not found, attempting to create...")
		_, err = p.pool.Exec(ctx, "CREATE EXTENSION vector")
		if err != nil {
			return fmt.Errorf("failed to create pgvector extension: %w. Please ensure pgvector is installed for your PostgreSQL version", err)
		}
		g.Log().Infof(ctx, "pgvector extension created successfully")
	} else {
		g.Log().Infof(ctx, "pgvector extension already exists")
	}

	// 2. 创建独立的 vectors schema
	_, err = p.pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", p.schema))
	if err != nil {
		return fmt.Errorf("failed to create vectors schema: %w", err)
	}

	g.Log().Infof(ctx, "PostgreSQL database '%s' ready with pgvector extension and '%s' schema", p.database, p.schema)
	return nil
}

// CreateCollection 创建集合（表）- 使用模型定义
func (p *PostgresStore) CreateCollection(ctx context.Context, collectionName string, dimension int) error {
	// 清理表名，防止SQL注入
	tableName := p.sanitizeTableName(collectionName)

	// 使用标准表结构模型
	schema := pgvectorModel.TableSchema{}

	// 1. 创建表，使用传入的维度参数
	createTableSQL := schema.GenerateCreateTableSQL(p.schema, tableName, dimension)
	_, err := p.pool.Exec(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table %s.%s: %w", p.schema, tableName, err)
	}

	// 2. 创建索引
	createIndexSQLs := schema.GenerateCreateIndexSQL(p.schema, tableName)
	for _, indexSQL := range createIndexSQLs {
		_, err = p.pool.Exec(ctx, indexSQL)
		if err != nil {
			return fmt.Errorf("failed to create index on table %s.%s: %w", p.schema, tableName, err)
		}
	}

	g.Log().Infof(ctx, "Table '%s.%s' created with dimension %d and indexes", p.schema, tableName, dimension)
	return nil
}

// CollectionExists 检查集合（表）是否存在
func (p *PostgresStore) CollectionExists(ctx context.Context, collectionName string) (bool, error) {
	tableName := p.sanitizeTableName(collectionName)

	var exists bool
	err := p.pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)",
		p.schema, tableName,
	).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check if table %s.%s exists: %w", p.schema, tableName, err)
	}

	return exists, nil
}

// DeleteCollection 删除集合（表）
func (p *PostgresStore) DeleteCollection(ctx context.Context, collectionName string) error {
	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	_, err := p.pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", fullTableName))
	if err != nil {
		return fmt.Errorf("failed to drop table %s: %w", fullTableName, err)
	}

	g.Log().Infof(ctx, "Table '%s' deleted", fullTableName)
	return nil
}

// InsertVectors 插入向量数据
func (p *PostgresStore) InsertVectors(ctx context.Context, collectionName string, chunks []*schema.Document, vectors [][]float32) ([]string, error) {
	if len(chunks) != len(vectors) {
		return nil, fmt.Errorf("chunks and vectors length mismatch: %d vs %d", len(chunks), len(vectors))
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
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (id, text, vector, document_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, fullTableName)

	for idx, chunk := range chunks {
		// 生成chunk ID（如果不存在）
		if len(chunk.ID) == 0 {
			chunk.ID = uuid.New().String()
		}
		ids[idx] = chunk.ID

		// 截断文本（如果需要）
		text := p.truncateString(chunk.Content, 65535)

		// 转换向量为pgvector格式 - 直接使用float32向量
		pgVector := pgvector.NewVector(vectors[idx])

		// 设置document_id
		var docID string
		if contextDocumentId != "" {
			docID = contextDocumentId
		} else {
			return nil, fmt.Errorf("document_id not found in context for chunk %s", chunk.ID)
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
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}

		// 插入数据
		_, err = tx.Exec(ctx, insertSQL, chunk.ID, text, pgVector, docID, metaBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to insert vector for chunk %s: %w", chunk.ID, err)
		}
	}

	// 提交事务
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	g.Log().Infof(ctx, "Successfully inserted %d vectors into table '%s'", len(chunks), fullTableName)
	return ids, nil
}

// DeleteByDocumentID 根据文档ID删除所有相关chunks
func (p *PostgresStore) DeleteByDocumentID(ctx context.Context, collectionName string, documentID string) error {
	// 验证 documentID 格式
	if !common.ValidateUUID(documentID) {
		return fmt.Errorf("invalid document ID format: %s (must be valid UUID)", documentID)
	}

	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	g.Log().Infof(ctx, "Deleting all chunks of document %s from table %s", documentID, fullTableName)

	result, err := p.pool.Exec(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE document_id = $1", fullTableName),
		documentID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete document %s: %w", documentID, err)
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
		return fmt.Errorf("invalid chunk ID format: %s (must be valid UUID)", chunkID)
	}

	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	g.Log().Infof(ctx, "Deleting chunk %s from table %s", chunkID, fullTableName)

	result, err := p.pool.Exec(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE id = $1", fullTableName),
		chunkID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete chunk %s: %w", chunkID, err)
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
func (p *PostgresStore) NewRetriever(ctx context.Context, conf interface{}, collectionName string) (Retriever, error) {
	if p.pool == nil {
		return nil, fmt.Errorf("postgres pool not provided")
	}

	if collectionName == "" {
		return nil, fmt.Errorf("collection name cannot be empty")
	}

	tableName := p.sanitizeTableName(collectionName)
	fullTableName := fmt.Sprintf("%s.%s", p.schema, tableName)

	// 检查表是否存在
	exists, err := p.CollectionExists(ctx, collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("table '%s' not found", fullTableName)
	}

	// 创建并返回检索器
	return &postgresRetriever{
		pool:      p.pool,
		tableName: fullTableName, // 使用带 schema 的完整表名
		config:    conf,
	}, nil
}

// VectorSearchOnly 仅使用向量检索的通用方法
func (p *PostgresStore) VectorSearchOnly(ctx context.Context, conf GeneralRetrieverConfig, query string, knowledgeId string, topK int, score float64) ([]*schema.Document, error) {
	// knowledge name == table name
	tableName := p.sanitizeTableName(knowledgeId)

	// 创建检索器
	r, err := p.NewRetriever(ctx, conf, knowledgeId)
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

	return nil, fmt.Errorf("failed to cast retriever to postgresRetriever")
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

// postgresRetriever 实现了 retriever.Retriever 接口
type postgresRetriever struct {
	pool      *pgxpool.Pool
	tableName string
	config    interface{}
}

// Retrieve 实现检索功能
func (r *postgresRetriever) Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error) {
	// 默认参数
	topK := 5

	// 解析选项（这里简化处理，实际使用时需要更完整的选项解析）
	_ = opts // 暂时忽略选项

	return r.vectorSearchWithThreshold(ctx, query, topK, 0.0)
}

// vectorSearchWithThreshold 带阈值的向量搜索
func (r *postgresRetriever) vectorSearchWithThreshold(ctx context.Context, query string, topK int, threshold float64) ([]*schema.Document, error) {
	// 获取embedding配置 - 使用接口方法获取,避免循环依赖
	var apiKey, baseURL, embeddingModel string
	if r.config != nil {
		// 尝试通过接口方法获取配置
		type embeddingConfigGetter interface {
			GetAPIKey() string
			GetBaseURL() string
			GetEmbeddingModel() string
		}

		if configGetter, ok := r.config.(embeddingConfigGetter); ok {
			apiKey = configGetter.GetAPIKey()
			baseURL = configGetter.GetBaseURL()
			embeddingModel = configGetter.GetEmbeddingModel()
		} else {
			// Fallback: 尝试使用 map 获取配置字段
			if configMap, ok := r.config.(map[string]interface{}); ok {
				if key, exists := configMap["apiKey"]; exists {
					apiKey = fmt.Sprintf("%v", key)
				}
				if url, exists := configMap["baseURL"]; exists {
					baseURL = fmt.Sprintf("%v", url)
				}
				if model, exists := configMap["embeddingModel"]; exists {
					embeddingModel = fmt.Sprintf("%v", model)
				}
			}
		}
	}

	// 创建embedding配置
	embeddingConfig := &embeddingConfigWrapper{
		apiKey:         apiKey,
		baseURL:        baseURL,
		embeddingModel: embeddingModel,
	}

	// 创建embedder
	embedder, err := common.NewEmbedding(ctx, embeddingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	// 生成查询向量
	// 获取向量维度，优先从配置文件读取
	dim := g.Cfg().MustGet(ctx, "postgres.dim", 1024).Int()
	vectors, err := embedder.EmbedStrings(ctx, []string{query}, dim)
	if err != nil {
		return nil, fmt.Errorf("embedding has error: %w", err)
	}

	if len(vectors) != 1 {
		return nil, fmt.Errorf("invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	// 直接使用float32向量
	queryVector := pgvector.NewVector(vectors[0])

	// 获取距离度量类型，从配置文件读取
	metricType := g.Cfg().MustGet(ctx, "vectordb.metricType", "COSINE").String()

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
	searchSQL := fmt.Sprintf(`
		SELECT id, text, document_id, metadata,
		       %s as similarity_score
		FROM %s
		WHERE %s >= $2
		ORDER BY %s
		LIMIT $3
	`, scoreCalc, r.tableName, scoreCalc, orderBy)

	rows, err := r.pool.Query(ctx, searchSQL, queryVector, threshold, topK)
	if err != nil {
		return nil, fmt.Errorf("failed to execute vector search: %w", err)
	}
	defer rows.Close()

	var results []*schema.Document
	for rows.Next() {
		var id, text, documentId string
		var metadataBytes []byte
		var score float64

		err := rows.Scan(&id, &text, &documentId, &metadataBytes, &score)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
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
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	// 权限控制：过滤掉status != 1的chunks
	if len(results) > 0 {
		chunkIDs := make([]string, 0, len(results))
		for _, doc := range results {
			chunkIDs = append(chunkIDs, doc.ID)
		}

		activeIDs, err := dao.KnowledgeChunks.GetActiveChunkIDs(ctx, chunkIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to query chunk status: %w", err)
		}

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
			if activeIDs.Contains(doc.ID) {
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
