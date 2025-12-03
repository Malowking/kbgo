package vector_store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/dao"
	milvusModel "github.com/Malowking/kbgo/internal/model/milvus"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// MilvusStore Milvus向量数据库实现
type MilvusStore struct {
	client   *milvusclient.Client
	database string
}

// embeddingConfigWrapper 实现 EmbeddingConfig 接口的包装器
type embeddingConfigWrapper struct {
	apiKey         string
	baseURL        string
	embeddingModel string
}

func (e *embeddingConfigWrapper) GetAPIKey() string         { return e.apiKey }
func (e *embeddingConfigWrapper) GetBaseURL() string        { return e.baseURL }
func (e *embeddingConfigWrapper) GetEmbeddingModel() string { return e.embeddingModel }

func InitializeMilvusStore(ctx context.Context) (VectorStore, error) {
	address := g.Cfg().MustGet(ctx, "milvus.address", "").String()
	database := g.Cfg().MustGet(ctx, "milvus.database", "default").String()

	if address == "" {
		return nil, fmt.Errorf("milvus.address is required but not found in config file. Please check your config.yaml file and ensure milvus.address is properly set")
	}

	g.Log().Infof(ctx, "Connecting to Milvus at: %s, database: %s", address, database)

	// Create Milvus client directly using milvusclient
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: address,
		DBName:  database,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus client (address: %s, database: %s): %w", address, database, err)
	}

	// Create MilvusStore with the client
	config := &VectorStoreConfig{
		Type:     VectorStoreTypeMilvus,
		Client:   client,
		Database: database,
	}

	milvusStore, err := NewMilvusStore(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus store: %w", err)
	}

	return milvusStore, nil
}

// NewMilvusStore 创建Milvus向量存储实例
func NewMilvusStore(config *VectorStoreConfig) (VectorStore, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	client, ok := config.Client.(*milvusclient.Client)
	if !ok {
		return nil, fmt.Errorf("client must be *milvusclient.Client")
	}

	if config.Database == "" {
		return nil, fmt.Errorf("database name cannot be empty")
	}

	return &MilvusStore{
		client:   client,
		database: config.Database,
	}, nil
}

// CreateDatabaseIfNotExists 创建数据库（如果不存在）
func (m *MilvusStore) CreateDatabaseIfNotExists(ctx context.Context) error {
	dbNames, err := m.client.ListDatabase(ctx, milvusclient.NewListDatabaseOption())
	if err != nil {
		return fmt.Errorf("failed to list databases: %w", err)
	}

	for _, name := range dbNames {
		if strings.EqualFold(name, m.database) {
			g.Log().Infof(ctx, "Database '%s' already exists, skipping creation", m.database)
			return nil
		}
	}

	// 数据库不存在，创建
	err = m.client.CreateDatabase(ctx, milvusclient.NewCreateDatabaseOption(m.database))
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	g.Log().Infof(ctx, "Database '%s' created successfully", m.database)
	return nil
}

// CreateCollection 创建集合
func (m *MilvusStore) CreateCollection(ctx context.Context, collectionName string) error {
	// 获取向量维度，优先从配置文件读取
	dim := g.Cfg().MustGet(ctx, "milvus.dim", 1024).Int()
	dimStr := fmt.Sprintf("%d", dim)

	// 使用标准 text collection schema
	schema := &entity.Schema{
		CollectionName: collectionName,
		Description:    "存储文档分片及其向量",
		AutoID:         false,
		Fields:         milvusModel.GetStandardCollectionFields(dimStr),
	}

	// 创建文档片段集合，并设置vector为索引
	err := m.client.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(collectionName, schema).WithIndexOptions(
		milvusclient.NewCreateIndexOption(collectionName, "vector", index.NewHNSWIndex(entity.L2, 64, 128))))
	if err != nil {
		return fmt.Errorf("failed to create Milvus collection: %w", err)
	}

	// Load collection into memory
	_, err = m.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(collectionName))
	if err != nil {
		return fmt.Errorf("failed to load Milvus collection: %w", err)
	}

	g.Log().Infof(ctx, "Collection '%s' created with dimension %d, index built and loaded", collectionName, dim)
	return nil
}

// CollectionExists 检查集合是否存在
func (m *MilvusStore) CollectionExists(ctx context.Context, collectionName string) (bool, error) {
	has, err := m.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(collectionName))
	if err != nil {
		return false, fmt.Errorf("failed to check if collection exists: %w", err)
	}
	return has, nil
}

// DeleteCollection 删除集合
func (m *MilvusStore) DeleteCollection(ctx context.Context, collectionName string) error {
	err := m.client.DropCollection(ctx, milvusclient.NewDropCollectionOption(collectionName))
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	g.Log().Infof(ctx, "Collection '%s' deleted", collectionName)
	return nil
}

// InsertVectors 插入向量数据 - 直接使用float32向量
func (m *MilvusStore) InsertVectors(ctx context.Context, collectionName string, chunks []*schema.Document, vectors [][]float32) ([]string, error) {
	if len(chunks) != len(vectors) {
		return nil, fmt.Errorf("chunks and vectors length mismatch: %d vs %d", len(chunks), len(vectors))
	}

	ids := make([]string, len(chunks))
	texts := make([]string, len(chunks))
	documentIds := make([]string, len(chunks))
	metadataList := make([][]byte, len(chunks))

	// 从上下文中提取knowledge_id
	var knowledgeId string
	if value, ok := ctx.Value(common.KnowledgeId).(string); ok {
		knowledgeId = value
	}

	// 从上下文中提取document_id
	var contextDocumentId string
	if value, ok := ctx.Value(common.DocumentId).(string); ok {
		contextDocumentId = value
	}

	for idx, chunk := range chunks {
		// 生成chunk ID（如果不存在）
		if len(chunk.ID) == 0 {
			chunk.ID = uuid.New().String()
		}
		ids[idx] = chunk.ID

		// 截断文本（如果需要）
		texts[idx] = truncateString(chunk.Content, 65535)

		// 设置document_id
		var docID string
		if contextDocumentId != "" {
			docID = contextDocumentId
		} else {
			return nil, fmt.Errorf("document_id not found in context for chunk %s", chunk.ID)
		}
		documentIds[idx] = docID

		// 构建metadata
		metaCopy := make(map[string]any)
		if chunk.MetaData != nil {
			for k, v := range chunk.MetaData {
				metaCopy[k] = v
			}
		}

		// 添加knowledge_id到metadata
		if knowledgeId != "" {
			metaCopy[common.KnowledgeId] = knowledgeId
		}

		// 序列化metadata
		metaBytes, err := marshalMetadata(metaCopy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataList[idx] = metaBytes
	}

	// 获取向量维度，优先从配置文件读取
	dim := g.Cfg().MustGet(ctx, "milvus.dim", 1024).Int()

	// 创建列数据 - 直接使用传入的float32向量
	columns := []column.Column{
		column.NewColumnVarChar("id", ids),
		column.NewColumnVarChar("text", texts),
		column.NewColumnFloatVector("vector", dim, vectors),
		column.NewColumnVarChar("document_id", documentIds),
		column.NewColumnJSONBytes("metadata", metadataList),
	}

	// 插入数据
	insertOpt := milvusclient.NewColumnBasedInsertOption(collectionName, columns...)
	result, err := m.client.Insert(ctx, insertOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert vectors: %w", err)
	}

	g.Log().Infof(ctx, "Successfully inserted %d vectors into collection '%s'", result.InsertCount, collectionName)
	return ids, nil
}

// DeleteByDocumentID 根据文档ID删除所有相关chunks
func (m *MilvusStore) DeleteByDocumentID(ctx context.Context, collectionName string, documentID string) error {
	// 验证 documentID 格式（防止注入）
	if !common.ValidateUUID(documentID) {
		return fmt.Errorf("invalid document ID format: %s (must be valid UUID)", documentID)
	}

	// 转义特殊字符（双重保护）
	safeDocID := common.SanitizeMilvusString(documentID)
	filterExpr := fmt.Sprintf(`document_id == "%s"`, safeDocID)

	g.Log().Infof(ctx, "Deleting all chunks of document %s from collection %s", documentID, collectionName)

	deleteOpt := milvusclient.NewDeleteOption(collectionName).WithExpr(filterExpr)
	result, err := m.client.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("failed to delete document %s: %w", documentID, err)
	}

	g.Log().Infof(ctx, "Delete operation completed for document %s, affected rows: %d", documentID, result.DeleteCount)

	if result.DeleteCount == 0 {
		g.Log().Infof(ctx, "Warning: No chunks were deleted for document_id=%s", documentID)
	}

	return nil
}

// DeleteByChunkID 根据chunkID删除单个chunk
func (m *MilvusStore) DeleteByChunkID(ctx context.Context, collectionName string, chunkID string) error {
	// 验证 chunkID 格式（防止注入）
	if !common.ValidateUUID(chunkID) {
		return fmt.Errorf("invalid chunk ID format: %s (must be valid UUID)", chunkID)
	}

	// 转义特殊字符（双重保护）
	safeChunkID := common.SanitizeMilvusString(chunkID)
	filterExpr := fmt.Sprintf(`id == "%s"`, safeChunkID)

	g.Log().Infof(ctx, "Deleting chunk %s from collection %s", chunkID, collectionName)

	deleteOpt := milvusclient.NewDeleteOption(collectionName).WithExpr(filterExpr)
	result, err := m.client.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("failed to delete chunk %s: %w", chunkID, err)
	}

	g.Log().Infof(ctx, "Delete operation completed for chunk %s, affected rows: %d", chunkID, result.DeleteCount)

	if result.DeleteCount == 0 {
		g.Log().Infof(ctx, "Warning: No chunk was deleted for id=%s", chunkID)
	}

	return nil
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

// GetClient returns the underlying Milvus client as interface{}
func (m *MilvusStore) GetClient() interface{} {
	return m.client
}

// GetMilvusClient returns the underlying Milvus client with specific type
func (m *MilvusStore) GetMilvusClient() *milvusclient.Client {
	return m.client
}

// NewRetriever 创建检索器实例（通用方法名）
func (m *MilvusStore) NewRetriever(ctx context.Context, conf interface{}, collectionName string) (Retriever, error) {
	return m.NewMilvusRetriever(ctx, conf, collectionName)
}

// milvusRetriever 实现了 Retriever 接口
type milvusRetriever struct {
	client         *milvusclient.Client
	collectionName string
	vectorField    string
	config         interface{} // 使用 interface{} 避免循环导入
}

// Retrieve 实现检索功能
func (r *milvusRetriever) Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error) {
	// 使用反射获取配置字段值，避免循环导入
	topK := 5 // 默认值
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

	// 获取 Milvus 特定选项（filter, partition）
	var filter, partition string
	for _, opt := range opts {
		// 尝试应用到临时Options来提取filter和partition
		tempOpts := &Options{}
		opt(tempOpts)
		if tempOpts.Filter != "" {
			filter = tempOpts.Filter
		}
		if tempOpts.Partition != "" {
			partition = tempOpts.Partition
		}
	}

	// 创建embedding实例 - 使用接口方法获取配置,避免反射
	var apiKey, baseURL, embeddingModel string
	if r.config != nil {
		// 定义接口
		type embeddingConfigGetter interface {
			GetAPIKey() string
			GetBaseURL() string
			GetEmbeddingModel() string
		}

		// 尝试通过接口方法获取配置
		if configGetter, ok := r.config.(embeddingConfigGetter); ok {
			apiKey = configGetter.GetAPIKey()
			baseURL = configGetter.GetBaseURL()
			embeddingModel = configGetter.GetEmbeddingModel()
		} else {
			// Fallback: 尝试使用反射获取配置字段(兼容旧代码)
			configValue := reflect.ValueOf(r.config)
			if configValue.Kind() == reflect.Ptr {
				configValue = configValue.Elem()
			}
			if configValue.Kind() == reflect.Struct {
				// 获取 APIKey
				if apiKeyField := configValue.FieldByName("APIKey"); apiKeyField.IsValid() && apiKeyField.CanInterface() {
					if key, ok := apiKeyField.Interface().(string); ok {
						apiKey = key
					}
				}
				// 获取 BaseURL
				if baseURLField := configValue.FieldByName("BaseURL"); baseURLField.IsValid() && baseURLField.CanInterface() {
					if url, ok := baseURLField.Interface().(string); ok {
						baseURL = url
					}
				}
				// 获取 EmbeddingModel
				if modelField := configValue.FieldByName("EmbeddingModel"); modelField.IsValid() && modelField.CanInterface() {
					if model, ok := modelField.Interface().(string); ok {
						embeddingModel = model
					}
				}
			}
		}
	}

	// 创建一个临时的配置结构来满足 EmbeddingConfig 接口
	embeddingConfig := &embeddingConfigWrapper{
		apiKey:         apiKey,
		baseURL:        baseURL,
		embeddingModel: embeddingModel,
	}

	embedder, err := common.NewEmbedding(ctx, embeddingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	// embedding查询 - 直接获取float32向量
	// 获取向量维度，优先从配置文件读取
	dim := g.Cfg().MustGet(ctx, "milvus.dim", 1024).Int()
	vectors, err := embedder.EmbedStrings(ctx, []string{query}, dim)
	if err != nil {
		return nil, fmt.Errorf("embedding has error: %w", err)
	}
	// 检查 embedding result
	if len(vectors) != 1 {
		return nil, fmt.Errorf("invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	// 将float32向量转换为entity.Vector
	entityVectors := make([]entity.Vector, len(vectors))
	for i, vec := range vectors {
		entityVectors[i] = entity.FloatVector(vec)
	}

	// 准备分区
	partitions := []string{}
	if partition != "" {
		partitions = []string{partition}
	}

	// 准备搜索选项
	searchOpt := milvusclient.NewSearchOption(r.collectionName, topK, entityVectors).
		WithANNSField(r.vectorField).
		WithOutputFields("id", "text", "document_id", "metadata").
		WithConsistencyLevel(entity.ClBounded)

	// 添加分区如果提供
	if len(partitions) > 0 {
		searchOpt = searchOpt.WithPartitions(partitions...)
	}

	// 添加过滤条件如果提供
	if filter != "" {
		searchOpt = searchOpt.WithFilter(filter)
	}

	// 搜索集合
	results, err := r.client.Search(ctx, searchOpt)
	if err != nil {
		return nil, fmt.Errorf("search has error: %w", err)
	}

	// 检查搜索结果
	if len(results) == 0 {
		return []*schema.Document{}, nil
	}

	// 转换搜索结果为schema.Document - 复用原有转换逻辑
	return r.convertResultsToDocuments(ctx, results[0].Fields, results[0].Scores)
}

// convertResultsToDocuments 转换搜索结果为文档
func (r *milvusRetriever) convertResultsToDocuments(ctx context.Context, columns []column.Column, scores []float32) ([]*schema.Document, error) {
	if len(columns) == 0 {
		return nil, nil
	}

	// 确定文档数量
	numDocs := columns[0].Len()
	result := make([]*schema.Document, numDocs)
	for i := range result {
		result[i] = &schema.Document{
			MetaData: make(map[string]any),
		}
	}

	// 设置分数
	for i := 0; i < numDocs && i < len(scores); i++ {
		result[i].Score = scores[i]
	}

	// 处理各列数据
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
		case "text":
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					return nil, fmt.Errorf("failed to get text: %w", err)
				}
				if str, ok := val.(string); ok {
					result[i].Content = str
				}
			}
		case "metadata":
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					continue
				}
				if val == nil {
					continue
				}

				// 处理JSON格式的metadata
				switch v := val.(type) {
				case string:
					var metadata map[string]any
					if err := json.Unmarshal([]byte(v), &metadata); err == nil {
						for k, mv := range metadata {
							result[i].MetaData[k] = mv
						}
					}
				case []byte:
					var metadata map[string]any
					if err := json.Unmarshal(v, &metadata); err == nil {
						for k, mv := range metadata {
							result[i].MetaData[k] = mv
						}
					}
				}
			}
		default:
			// 其他字段添加到metadata
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

// GetType 返回检索器类型
func (r *milvusRetriever) GetType() string {
	return "MilvusRetriever"
}

// IsCallbacksEnabled 返回是否启用回调
func (r *milvusRetriever) IsCallbacksEnabled() bool {
	return false
}

// NewMilvusRetriever 创建Milvus检索器实例
func (m *MilvusStore) NewMilvusRetriever(ctx context.Context, conf interface{}, collectionName string) (Retriever, error) {
	if m.client == nil {
		return nil, fmt.Errorf("milvus client not provided")
	}

	if collectionName == "" {
		return nil, fmt.Errorf("collection name cannot be empty")
	}

	// 检查集合是否存在
	hasCollectionOpt := milvusclient.NewHasCollectionOption(collectionName)
	exists, err := m.client.HasCollection(ctx, hasCollectionOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' not found", collectionName)
	}

	// 获取集合描述
	descCollOpt := milvusclient.NewDescribeCollectionOption(collectionName)
	collection, err := m.client.DescribeCollection(ctx, descCollOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to describe collection: %w", err)
	}

	// 检查向量字段是否存在
	vectorField := "vector" // 默认向量字段名
	found := false
	for _, field := range collection.Schema.Fields {
		if field.Name == vectorField {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("vector field '%s' not found in collection schema", vectorField)
	}

	// 确保集合已加载
	if !collection.Loaded {
		loadOpt := milvusclient.NewLoadCollectionOption(collectionName)
		_, err = m.client.LoadCollection(ctx, loadOpt)
		if err != nil {
			return nil, fmt.Errorf("failed to load collection: %w", err)
		}
	}

	// 创建并返回检索器
	return &milvusRetriever{
		client:         m.client,
		collectionName: collectionName,
		vectorField:    vectorField,
		config:         conf,
	}, nil
}

// ConvertSearchResultsToDocuments converts Milvus search results to schema.Document
// and filters out chunks with status != 1 for permission control
func (m *MilvusStore) ConvertSearchResultsToDocuments(ctx context.Context, columns []column.Column, scores []float32) ([]*schema.Document, error) {
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
		result[i].Score = scores[i]
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
		case common.FieldContent: // "text"
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
				_, err := col.Get(i)
				if err != nil {
					return nil, fmt.Errorf("failed to get content_vector: %w", err)
				}
				// Milvus returns vectors as []float32 or []byte - we don't need to store them in the document
				// The vectors are only used for similarity search, not for retrieval
			}
		case common.FieldMetadata: // "metadata" - consolidate JSON parsing logic
			for i := 0; i < col.Len(); i++ {
				val, err := col.Get(i)
				if err != nil {
					continue
				}
				if val == nil {
					continue
				}
				// Handle both string and []byte for metadata field - always parse as JSON
				switch v := val.(type) {
				case string:
					// Parse JSON string to map
					var metadata map[string]any
					if err := json.Unmarshal([]byte(v), &metadata); err == nil {
						for k, mv := range metadata {
							result[i].MetaData[k] = mv
						}
					}
				case []byte:
					// Parse JSON bytes to map
					var metadata map[string]any
					if err := json.Unmarshal(v, &metadata); err == nil {
						for k, mv := range metadata {
							result[i].MetaData[k] = mv
						}
					}
				}
			}
		case common.DocumentId: // "document_id"
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

// VectorSearchOnly 仅使用向量检索的通用方法
func (m *MilvusStore) VectorSearchOnly(ctx context.Context, conf GeneralRetrieverConfig, query string, knowledgeId string, topK int, score float64) ([]*schema.Document, error) {
	var filter string
	// knowledge name == collection name
	collectionName := knowledgeId

	// 直接传入配置接口，让 NewMilvusRetriever 内部处理
	r, err := m.NewMilvusRetriever(ctx, conf, collectionName)
	if err != nil {
		g.Log().Errorf(ctx, "failed to create retriever for collection %s, err=%v", collectionName, err)
		return nil, err
	}

	// Milvus 检索的 TopK，可以设置得比最终需要的数量大一些
	// 因为后续会经过 rerank 重新排序
	milvusTopK := topK * 5 // 取5倍数量，给 rerank 更多选择空间
	if milvusTopK < 20 {
		milvusTopK = 20 // 至少取20个
	}

	// 执行检索
	var options []Option
	options = append(options, WithTopK(milvusTopK))

	// 只有在有过滤条件时才添加 filter
	if filter != "" {
		options = append(options, WithFilter(filter))
	}

	docs, err := r.Retrieve(ctx, query, options...)
	if err != nil {
		return nil, err
	}

	// 归一化Milvus的COSINE分数（0-2范围）到标准的0-1范围
	// Milvus COSINE分数含义：0=完全相反, 1=正交, 2=完全相同
	// 归一化后：0=完全相反, 0.5=正交, 1=完全相同
	for _, s := range docs {
		normalizedScore := s.Score / 2.0
		s.Score = normalizedScore
	}

	// 去重
	docs = common.RemoveDuplicates(docs, func(doc *schema.Document) string {
		return doc.ID
	})

	// 按照向量相似度排序并截取 TopK
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Score > docs[j].Score
	})
	if len(docs) > topK {
		docs = docs[:topK]
	}

	// 过滤低分文档
	var relatedDocs []*schema.Document
	for _, doc := range docs {
		if doc.Score < float32(score) {
			g.Log().Debugf(ctx, "score less: %v, related: %v", doc.Score, doc.Content)
			continue
		}
		relatedDocs = append(relatedDocs, doc)
	}

	return relatedDocs, nil
}
