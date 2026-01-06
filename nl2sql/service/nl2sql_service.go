package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/core/vector_store"
	internalService "github.com/Malowking/kbgo/internal/service"
	"github.com/Malowking/kbgo/nl2sql/adapter"
	"github.com/Malowking/kbgo/nl2sql/cache"
	nl2sqlCommon "github.com/Malowking/kbgo/nl2sql/common"
	"github.com/Malowking/kbgo/nl2sql/datasource"
	"github.com/Malowking/kbgo/nl2sql/generator"
	"github.com/Malowking/kbgo/nl2sql/parser"
	"github.com/Malowking/kbgo/nl2sql/schema"
	"github.com/Malowking/kbgo/nl2sql/vector"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// NL2SQLService NL2SQL核心服务
type NL2SQLService struct {
	db            *gorm.DB
	redis         *redis.Client
	taskCache     *cache.TaskCache
	sqlValidator  *parser.SQLValidator
	schemaBuilder *schema.SchemaBuilder
}

// NewNL2SQLService 创建NL2SQL服务
func NewNL2SQLService(db *gorm.DB, redisClient *redis.Client) *NL2SQLService {
	return &NL2SQLService{
		db:            db,
		redis:         redisClient,
		taskCache:     cache.NewTaskCache(redisClient),
		sqlValidator:  parser.NewSQLValidator(),
		schemaBuilder: schema.NewSchemaBuilder(db),
	}
}

// CreateDataSourceRequest 创建数据源请求
type CreateDataSourceRequest struct {
	Name             string                 `json:"name"`
	Type             string                 `json:"type"` // jdbc, csv, excel
	DBType           string                 `json:"db_type"`
	Config           map[string]interface{} `json:"config"`
	CreatedBy        string                 `json:"created_by"`
	EmbeddingModelID string                 `json:"embedding_model_id"` // 新增：用于创建向量表
}

// CreateDataSourceResponse 创建数据源响应
type CreateDataSourceResponse struct {
	DatasourceID     string `json:"datasource_id"`
	Status           string `json:"status"`
	CollectionName   string `json:"collection_name"`
	VectorStoreReady bool   `json:"vector_store_ready"`
}

// CreateDataSource 创建数据源
func (s *NL2SQLService) CreateDataSource(ctx context.Context, req *CreateDataSourceRequest) (*CreateDataSourceResponse, error) {
	g.Log().Infof(ctx, "开始创建数据源 - Name: %s, Type: %s, EmbeddingModelID: %s", req.Name, req.Type, req.EmbeddingModelID)

	// 1. 验证embedding模型
	if req.EmbeddingModelID == "" {
		return nil, fmt.Errorf("embedding模型ID不能为空")
	}

	modelConfig := model.Registry.Get(req.EmbeddingModelID)
	if modelConfig == nil {
		return nil, fmt.Errorf("embedding模型不存在: %s", req.EmbeddingModelID)
	}

	if modelConfig.Type != model.ModelTypeEmbedding {
		return nil, fmt.Errorf("模型 %s 不是embedding模型，类型为: %s", req.EmbeddingModelID, modelConfig.Type)
	}

	// 2. 验证配置
	if req.Type == nl2sqlCommon.DataSourceTypeJDBC {
		if err := s.validateJDBCConfig(req.Config); err != nil {
			return nil, fmt.Errorf("JDBC配置无效: %w", err)
		}
	}

	// 3. 创建数据源记录
	configJSON, _ := json.Marshal(req.Config)
	ds := &dbgorm.NL2SQLDataSource{
		Name:             req.Name,
		Type:             req.Type,
		DBType:           req.DBType,
		Config:           configJSON,
		ReadOnly:         true,
		Status:           nl2sqlCommon.DataSourceStatusPending,
		CreatedBy:        &req.CreatedBy,
		EmbeddingModelID: req.EmbeddingModelID, // 保存embedding模型ID
	}

	if err := s.db.Create(ds).Error; err != nil {
		return nil, fmt.Errorf("创建数据源失败: %w", err)
	}

	g.Log().Infof(ctx, "数据源记录已创建 - ID: %s", ds.ID)

	// 4. 如果是JDBC，测试连接
	if req.Type == nl2sqlCommon.DataSourceTypeJDBC {
		if err := s.testJDBCConnection(ctx, req.Config); err != nil {
			// 连接失败，删除数据源记录（回滚）
			s.db.Delete(ds)
			return nil, fmt.Errorf("数据库连接测试失败: %w", err)
		}
	}

	// 5. 创建向量表（原子操作的关键部分）
	collectionName, err := s.createVectorCollection(ctx, ds.ID, req.EmbeddingModelID)
	if err != nil {
		// 向量表创建失败，删除数据源记录（回滚）
		g.Log().Errorf(ctx, "向量表创建失败，回滚数据源: %v", err)
		s.db.Delete(ds)
		return nil, fmt.Errorf("创建向量表失败: %w", err)
	}

	g.Log().Infof(ctx, "向量表创建成功 - Collection: %s", collectionName)

	// 6. 更新数据源状态为active
	ds.Status = nl2sqlCommon.DataSourceStatusActive
	if err := s.db.Save(ds).Error; err != nil {
		// 状态更新失败，删除向量表和数据源记录（回滚）
		g.Log().Errorf(ctx, "更新数据源状态失败，回滚: %v", err)
		s.DeleteVectorCollection(ctx, collectionName)
		s.db.Delete(ds)
		return nil, fmt.Errorf("更新数据源状态失败: %w", err)
	}

	g.Log().Infof(ctx, "数据源创建成功 - ID: %s, Collection: %s", ds.ID, collectionName)

	return &CreateDataSourceResponse{
		DatasourceID:     ds.ID,
		Status:           ds.Status,
		CollectionName:   collectionName,
		VectorStoreReady: true,
	}, nil
}

// AddTableToDataSource 向数据源添加表（用于CSV/Excel数据源）
func (s *NL2SQLService) AddTableToDataSource(ctx context.Context, datasourceID, tableName string) error {
	// 1. 获取数据源
	var ds dbgorm.NL2SQLDataSource
	if err := s.db.First(&ds, "id = ?", datasourceID).Error; err != nil {
		return fmt.Errorf("数据源不存在: %w", err)
	}

	// 2. 验证数据源类型（只支持 CSV/Excel）
	if ds.Type != nl2sqlCommon.DataSourceTypeCSV && ds.Type != nl2sqlCommon.DataSourceTypeExcel {
		return fmt.Errorf("只有 CSV/Excel 类型的数据源支持添加表，当前类型: %s", ds.Type)
	}

	// 3. 检查表名是否已存在
	var existingTable dbgorm.NL2SQLTable
	if err := s.db.Where("datasource_id = ? AND table_name = ?", datasourceID, tableName).First(&existingTable).Error; err == nil {
		return fmt.Errorf("表 %s 已存在于数据源中", tableName)
	}
	g.Log().Infof(ctx, "Table %s is ready to be added to datasource %s", tableName, datasourceID)
	return nil
}

// ParseDataSourceSchemaWithModels 解析数据源Schema
func (s *NL2SQLService) ParseDataSourceSchemaWithModels(ctx context.Context, datasourceID, llmModelID, embeddingModelID string) (string, error) {
	// 1. 获取数据源
	var ds dbgorm.NL2SQLDataSource
	if err := s.db.First(&ds, "id = ?", datasourceID).Error; err != nil {
		return "", fmt.Errorf("数据源不存在: %w", err)
	}

	// 2. 创建任务ID
	taskID := uuid.New().String()

	// 3. 保存任务到Redis
	task := &cache.TaskStatus{
		TaskID:      taskID,
		TaskType:    nl2sqlCommon.TaskTypeSchemaParam,
		Status:      nl2sqlCommon.TaskStatusPending,
		Progress:    0,
		CurrentStep: "等待执行",
		StartedAt:   time.Now(),
	}

	if err := s.taskCache.SaveTask(ctx, task); err != nil {
		return "", fmt.Errorf("创建任务失败: %w", err)
	}

	// 4. 启动后台任务（使用goroutine），传递模型参数
	go s.executeSchemaParseTaskWithModels(context.Background(), taskID, datasourceID, llmModelID, embeddingModelID)

	return taskID, nil
}

// executeSchemaParseTaskWithModels 执行Schema解析任务（带模型参数）
func (s *NL2SQLService) executeSchemaParseTaskWithModels(ctx context.Context, taskID, datasourceID, llmModelID, embeddingModelID string) {
	g.Log().Infof(ctx, "[Task %s] Schema parse task started for datasource %s", taskID, datasourceID)

	// 更新任务状态为running
	if err := s.taskCache.UpdateProgress(ctx, taskID, 10, "获取数据源信息"); err != nil {
		g.Log().Errorf(ctx, "[Task %s] Failed to update progress: %v", taskID, err)
	}

	// 1. 获取数据源
	var ds dbgorm.NL2SQLDataSource
	if err := s.db.First(&ds, "id = ?", datasourceID).Error; err != nil {
		g.Log().Errorf(ctx, "[Task %s] Failed to get datasource: %v", taskID, err)
		s.taskCache.MarkFailed(ctx, taskID, fmt.Sprintf("获取数据源失败: %v", err))
		return
	}
	g.Log().Infof(ctx, "[Task %s] Datasource found: %s (type: %s)", taskID, ds.Name, ds.Type)

	// 2. 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal(ds.Config, &config); err != nil {
		g.Log().Errorf(ctx, "[Task %s] Failed to parse config: %v", taskID, err)
		s.taskCache.MarkFailed(ctx, taskID, fmt.Sprintf("解析配置失败: %v", err))
		return
	}
	g.Log().Infof(ctx, "[Task %s] Config parsed successfully", taskID)

	var schemaID string
	var err error

	// 3. 根据数据源类型选择不同的Schema构建方式
	if ds.Type == nl2sqlCommon.DataSourceTypeCSV || ds.Type == nl2sqlCommon.DataSourceTypeExcel {
		// CSV/Excel文件：从nl2sql schema中读取已导入的表
		s.taskCache.UpdateProgress(ctx, taskID, 30, "从nl2sql schema提取元数据")

		// 从配置中读取所有表
		schemaID, err = s.schemaBuilder.BuildFromNL2SQLSchema(ctx, datasourceID)
		if err != nil {
			s.taskCache.MarkFailed(ctx, taskID, fmt.Sprintf("构建Schema失败: %v", err))
			return
		}

	} else if ds.Type == nl2sqlCommon.DataSourceTypeJDBC {
		// JDBC数据源：连接数据库提取Schema
		s.taskCache.UpdateProgress(ctx, taskID, 20, "连接数据库")

		dbConfig := mapToDBConfig(config, ds.DBType)
		connector := datasource.NewJDBCConnector(dbConfig)

		if err := connector.Connect(ctx); err != nil {
			s.taskCache.MarkFailed(ctx, taskID, fmt.Sprintf("连接数据库失败: %v", err))
			return
		}
		defer connector.Close()

		s.taskCache.UpdateProgress(ctx, taskID, 30, "提取Schema元数据")

		schemaID, err = s.schemaBuilder.BuildFromJDBC(ctx, datasourceID, connector)
		if err != nil {
			s.taskCache.MarkFailed(ctx, taskID, fmt.Sprintf("构建Schema失败: %v", err))
			return
		}
	} else {
		s.taskCache.MarkFailed(ctx, taskID, fmt.Sprintf("不支持的数据源类型: %s", ds.Type))
		return
	}

	s.taskCache.UpdateProgress(ctx, taskID, 60, "使用LLM增强描述")

	// 4. 使用LLM增强
	llmSuccess := true
	if llmModelID != "" {
		if err := s.enhanceSchemaWithLLM(ctx, datasourceID, llmModelID); err != nil {
			g.Log().Warningf(ctx, "[Task %s] LLM增强失败: %v", taskID, err)
			llmSuccess = false
			// LLM增强失败不影响整体流程，只记录警告
		} else {
			g.Log().Infof(ctx, "[Task %s] LLM增强Schema成功", taskID)
		}
	} else {
		g.Log().Warningf(ctx, "[Task %s] 未指定LLM模型，跳过增强", taskID)
	}

	s.taskCache.UpdateProgress(ctx, taskID, 80, "构建向量索引")

	// 5. 构建向量索引
	vectorizeSuccess := true
	if embeddingModelID != "" {
		if err := s.vectorizeSchema(ctx, datasourceID, embeddingModelID); err != nil {
			g.Log().Errorf(ctx, "[Task %s] 向量化失败: %v", taskID, err)
			vectorizeSuccess = false
			// 向量化失败是致命错误，标记任务失败
			s.taskCache.MarkFailed(ctx, taskID, fmt.Sprintf("向量化失败: %v", err))
			return
		} else {
			g.Log().Infof(ctx, "[Task %s] Schema向量化成功", taskID)
		}
	} else {
		g.Log().Warningf(ctx, "[Task %s] 未指定embedding模型，跳过向量化", taskID)
	}

	// 6. 所有步骤完成后，标记该数据源下所有表为已解析
	s.taskCache.UpdateProgress(ctx, taskID, 95, "更新解析状态")
	if err := s.db.Model(&dbgorm.NL2SQLTable{}).Where("datasource_id = ?", datasourceID).Update("parsed", true).Error; err != nil {
		g.Log().Errorf(ctx, "[Task %s] Failed to update tables parsed status: %v", taskID, err)
		s.taskCache.MarkFailed(ctx, taskID, fmt.Sprintf("更新解析状态失败: %v", err))
		return
	}
	g.Log().Infof(ctx, "[Task %s] All tables marked as parsed", taskID)

	// 7. 标记任务成功
	resultData := map[string]interface{}{
		"schema_id":          schemaID,
		"llm_model_id":       llmModelID,
		"embedding_model_id": embeddingModelID,
		"llm_enhanced":       llmSuccess,
		"vectorized":         vectorizeSuccess,
	}
	resultJSON, _ := json.Marshal(resultData)
	s.taskCache.MarkSuccess(ctx, taskID, string(resultJSON))
	g.Log().Infof(ctx, "[Task %s] Schema解析任务完成", taskID)
}

// QueryRequest 查询请求
type QueryRequest struct {
	DatasourceID string `json:"datasource_id"`
	Question     string `json:"question"`
	SessionID    string `json:"session_id"`
	LLMModelID   string `json:"llm_model_id"` // LLM模型ID
}

// QueryResponse 查询响应
type QueryResponse struct {
	QueryLogID  string       `json:"query_log_id"`
	SQL         string       `json:"sql"`
	Result      *QueryResult `json:"result"`
	Explanation string       `json:"explanation"`
	Error       string       `json:"error,omitempty"`
	FileURL     string       `json:"file_url,omitempty"` // 导出文件URL（当结果集较大时）
}

// QueryResult 查询结果
type QueryResult struct {
	Columns  []string                 `json:"columns"`
	Data     []map[string]interface{} `json:"data"`
	RowCount int                      `json:"row_count"`
}

// Query 执行NL2SQL查询（核心方法）
func (s *NL2SQLService) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	g.Log().Infof(ctx, "Query started - DatasourceID: %s, LLMModelID: %s", req.DatasourceID, req.LLMModelID)

	// 1. 验证LLM模型ID
	if req.LLMModelID == "" {
		return nil, fmt.Errorf("LLM模型ID不能为空")
	}

	// 2. 获取LLM模型配置
	llmModelConfig := model.Registry.Get(req.LLMModelID)
	if llmModelConfig == nil {
		return nil, fmt.Errorf("LLM模型不存在: %s", req.LLMModelID)
	}

	// 3. 获取数据源信息（用于获取embedding模型ID）
	var ds dbgorm.NL2SQLDataSource
	if err := s.db.First(&ds, "id = ?", req.DatasourceID).Error; err != nil {
		return nil, fmt.Errorf("数据源不存在: %w", err)
	}

	// 4. 创建LLM适配器
	llmAdapter := adapter.NewLLMAdapter(llmModelConfig)

	// 5. 创建向量搜索适配器（使用数据源的embedding模型）
	var vectorAdapter *adapter.VectorSearchAdapter
	if ds.EmbeddingModelID != "" {
		embeddingModelConfig := model.Registry.Get(ds.EmbeddingModelID)
		if embeddingModelConfig != nil {
			vectorStore, err := internalService.GetVectorStore()
			if err != nil {
				g.Log().Warningf(ctx, "获取向量存储失败: %v，将不使用向量搜索", err)
			} else {
				collectionName := fmt.Sprintf("nl2sql_%s", req.DatasourceID)
				vectorAdapter = adapter.NewVectorSearchAdapter(vectorStore, collectionName, req.DatasourceID, embeddingModelConfig)
			}
		}
	}

	// 6. 调用QueryWithAdapters执行查询
	return s.QueryWithAdapters(ctx, req, llmAdapter, vectorAdapter)
}

// 辅助方法

func (s *NL2SQLService) validateJDBCConfig(config map[string]interface{}) error {
	required := []string{"host", "port", "database", "username", "password"}
	for _, field := range required {
		if _, ok := config[field]; !ok {
			return fmt.Errorf("缺少必填字段: %s", field)
		}
	}
	return nil
}

func (s *NL2SQLService) testJDBCConnection(ctx context.Context, config map[string]interface{}) error {
	dbConfig := mapToDBConfig(config, config["db_type"].(string))
	connector := datasource.NewJDBCConnector(dbConfig)

	if err := connector.Connect(ctx); err != nil {
		return err
	}
	defer connector.Close()

	return connector.TestConnection(ctx)
}

func mapToDBConfig(config map[string]interface{}, dbType string) *datasource.DBConfig {
	return &datasource.DBConfig{
		DBType:   dbType,
		Host:     getString(config, "host"),
		Port:     getInt(config, "port"),
		Database: getString(config, "database"),
		Username: getString(config, "username"),
		Password: getString(config, "password"),
		SSLMode:  getString(config, "ssl_mode"),
	}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return 0
}

// vectorizeSchema 向量化Schema元数据
func (s *NL2SQLService) vectorizeSchema(ctx context.Context, datasourceID, embeddingModelID string) error {
	g.Log().Infof(ctx, "开始向量化Schema - DatasourceID: %s, EmbeddingModelID: %s", datasourceID, embeddingModelID)

	// 1. 获取embedding模型配置
	modelConfig := model.Registry.Get(embeddingModelID)
	if modelConfig == nil {
		return fmt.Errorf("embedding模型不存在: %s", embeddingModelID)
	}

	// 验证模型类型
	if modelConfig.Type != model.ModelTypeEmbedding {
		return fmt.Errorf("模型 %s 不是embedding模型，类型为: %s", embeddingModelID, modelConfig.Type)
	}

	// 2. 使用全局单例向量存储
	vectorStore, err := internalService.GetVectorStore()
	if err != nil {
		return fmt.Errorf("获取向量存储失败: %w", err)
	}

	// 3. 获取collection名称
	collectionName := fmt.Sprintf("nl2sql_%s", datasourceID)

	// 4. 检查集合是否存在
	exists, err := vectorStore.CollectionExists(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("集合不存在: %w", err)
	}

	if !exists {
		return fmt.Errorf("向量集合不存在: %s，请确保数据源创建时已正确创建向量表", collectionName)
	}

	g.Log().Infof(ctx, "向量集合已存在，开始向量化: %s", collectionName)

	// 5. 获取embedding维度（默认1024）
	dimension := 1024
	if dim, ok := modelConfig.Extra["dimension"].(float64); ok {
		dimension = int(dim)
	} else if dim, ok := modelConfig.Extra["dimension"].(int); ok {
		dimension = dim
	}

	// 6. 创建embedding客户端
	embedder, err := common.NewEmbedding(ctx, &embeddingConfigAdapter{
		apiKey:  modelConfig.APIKey,
		baseURL: modelConfig.BaseURL,
		model:   modelConfig.Name,
	})
	if err != nil {
		return fmt.Errorf("创建embedding客户端失败: %w", err)
	}

	// 7. 创建向量化器
	vectorizer := vector.NewSchemaVectorizer(s.db)

	// 8. 定义embedding函数
	embeddingFunc := func(text string) ([]float32, error) {
		// 调用embedding API
		vectors, err := embedder.EmbedStrings(ctx, []string{text}, dimension)
		if err != nil {
			return nil, fmt.Errorf("embedding失败: %w", err)
		}
		if len(vectors) == 0 || len(vectors[0]) == 0 {
			return nil, fmt.Errorf("embedding返回空向量")
		}
		return vectors[0], nil
	}

	// 9. 定义存储函数
	storeFunc := func(doc *vector.VectorDocument) error {
		// 从metadata中提取NL2SQL特定字段
		entityType := ""
		entityID := ""

		if et, ok := doc.Metadata["entity_type"].(string); ok {
			entityType = et
		}
		if eid, ok := doc.Metadata["entity_id"].(string); ok {
			entityID = eid
		}

		// 创建NL2SQLEntity
		entity := &vector_store.NL2SQLEntity{
			ID:           doc.ChunkID,
			EntityType:   entityType,
			EntityID:     entityID,
			DatasourceID: datasourceID,
			Text:         doc.Content,
			MetaData:     doc.Metadata,
		}

		// 使用NL2SQL专用插入方法
		_, err := vectorStore.InsertNL2SQLVectors(ctx, collectionName, []*vector_store.NL2SQLEntity{entity}, [][]float32{doc.Vector})
		return err
	}

	// 10. 执行向量化
	req := &vector.VectorizeSchemaRequest{
		DatasourceID:    datasourceID,
		KnowledgeBaseID: collectionName, // 使用collection名称作为知识库ID
		EmbeddingFunc:   embeddingFunc,
		StoreFunc:       storeFunc,
	}

	if err := vectorizer.VectorizeSchema(ctx, req); err != nil {
		return fmt.Errorf("向量化失败: %w", err)
	}

	g.Log().Infof(ctx, "Schema向量化完成 - DatasourceID: %s", datasourceID)
	return nil
}

// embeddingConfigAdapter embedding配置适配器
type embeddingConfigAdapter struct {
	apiKey  string
	baseURL string
	model   string
}

func (e *embeddingConfigAdapter) GetAPIKey() string {
	return e.apiKey
}

func (e *embeddingConfigAdapter) GetBaseURL() string {
	return e.baseURL
}

func (e *embeddingConfigAdapter) GetEmbeddingModel() string {
	return e.model
}

// enhanceSchemaWithLLM 使用LLM增强Schema描述
func (s *NL2SQLService) enhanceSchemaWithLLM(ctx context.Context, datasourceID, llmModelID string) error {
	g.Log().Infof(ctx, "开始LLM增强Schema - DatasourceID: %s, LLMModelID: %s", datasourceID, llmModelID)

	// 创建Schema增强器
	enhancer, err := schema.NewSchemaEnhancer(s.db, llmModelID)
	if err != nil {
		return fmt.Errorf("创建Schema增强器失败: %w", err)
	}

	// 执行增强
	req := &schema.EnhanceSchemaRequest{
		DatasourceID: datasourceID,
	}

	if err := enhancer.EnhanceSchema(ctx, req); err != nil {
		return fmt.Errorf("增强Schema失败: %w", err)
	}

	g.Log().Infof(ctx, "LLM增强Schema完成 - DatasourceID: %s", datasourceID)
	return nil
}

// createVectorCollection 创建向量集合（仅创建空集合，不进行向量化）
func (s *NL2SQLService) createVectorCollection(ctx context.Context, datasourceID, embeddingModelID string) (string, error) {
	g.Log().Infof(ctx, "创建向量集合 - DatasourceID: %s, EmbeddingModelID: %s", datasourceID, embeddingModelID)

	// 1. 获取embedding模型配置
	modelConfig := model.Registry.Get(embeddingModelID)
	if modelConfig == nil {
		return "", fmt.Errorf("embedding模型不存在: %s", embeddingModelID)
	}

	// 2. 使用全局单例向量存储
	vectorStore, err := internalService.GetVectorStore()
	if err != nil {
		return "", fmt.Errorf("获取向量存储失败: %w", err)
	}

	// 3. 生成集合名称
	collectionName := fmt.Sprintf("nl2sql_%s", datasourceID)

	// 4. 检查集合是否已存在
	exists, err := vectorStore.CollectionExists(ctx, collectionName)
	if err != nil {
		return "", fmt.Errorf("检查集合是否存在失败: %w", err)
	}

	if exists {
		g.Log().Warningf(ctx, "集合已存在，删除后重建: %s", collectionName)
		if err := vectorStore.DeleteCollection(ctx, collectionName); err != nil {
			return "", fmt.Errorf("删除旧集合失败: %w", err)
		}
	}

	// 5. 获取embedding维度（默认1024）
	dimension := 1024
	if dim, ok := modelConfig.Extra["dimension"].(float64); ok {
		dimension = int(dim)
	} else if dim, ok := modelConfig.Extra["dimension"].(int); ok {
		dimension = dim
	}

	// 6. 创建新集合
	if err := vectorStore.CreateCollection(ctx, collectionName, dimension); err != nil {
		return "", fmt.Errorf("创建集合失败: %w", err)
	}

	g.Log().Infof(ctx, "向量集合创建成功 - Collection: %s (维度: %d)", collectionName, dimension)
	return collectionName, nil
}

// DeleteVectorCollection 删除向量集合
func (s *NL2SQLService) DeleteVectorCollection(ctx context.Context, collectionName string) error {
	g.Log().Infof(ctx, "删除向量集合 - Collection: %s", collectionName)

	// 1. 使用全局单例向量存储
	vectorStore, err := internalService.GetVectorStore()
	if err != nil {
		g.Log().Errorf(ctx, "获取向量存储失败: %v", err)
		return fmt.Errorf("获取向量存储失败: %w", err)
	}

	// 2. 检查集合是否存在
	exists, err := vectorStore.CollectionExists(ctx, collectionName)
	if err != nil {
		g.Log().Errorf(ctx, "检查集合是否存在失败: %v", err)
		return fmt.Errorf("检查集合是否存在失败: %w", err)
	}

	if !exists {
		g.Log().Warningf(ctx, "集合不存在，跳过删除: %s", collectionName)
		return nil
	}

	// 3. 删除集合
	if err := vectorStore.DeleteCollection(ctx, collectionName); err != nil {
		g.Log().Errorf(ctx, "删除集合失败: %v", err)
		return fmt.Errorf("删除集合失败: %w", err)
	}

	g.Log().Infof(ctx, "向量集合删除成功 - Collection: %s", collectionName)
	return nil
}

// GetSQLGenerator 获取SQL生成器
func (s *NL2SQLService) GetSQLGenerator() *generator.SQLGenerator {
	return generator.NewSQLGenerator(s.db)
}
