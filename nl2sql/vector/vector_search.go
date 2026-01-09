package vector

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/core/vector_store"
	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/internal/service"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// NL2SQLVectorSearcher NL2SQL向量搜索器
type NL2SQLVectorSearcher struct {
	vectorStore vector_store.VectorStore
	db          *gorm.DB
}

// NewNL2SQLVectorSearcher 创建NL2SQL向量搜索器
func NewNL2SQLVectorSearcher(db *gorm.DB) (*NL2SQLVectorSearcher, error) {
	vectorStore, err := service.GetVectorStore()
	if err != nil {
		return nil, fmt.Errorf("获取向量存储失败: %w", err)
	}

	return &NL2SQLVectorSearcher{
		vectorStore: vectorStore,
		db:          db,
	}, nil
}

// SearchSchemaRequest Schema搜索请求
type SearchSchemaRequest struct {
	SchemaID string  // Schema ID（对应datasource_id）
	Query    string  // 用户问题
	TopK     int     // 返回结果数量，默认15
	MinScore float64 // 最小相似度分数，默认0.3
}

// SearchSchemaResponse Schema搜索响应
type SearchSchemaResponse struct {
	Results []*SchemaSearchResult `json:"results"`
}

// SchemaSearchResult Schema搜索结果
type SchemaSearchResult struct {
	DocumentID string                 `json:"document_id"` // 文档ID（对应entity_id）
	ChunkID    string                 `json:"chunk_id"`    // 块ID
	EntityType string                 `json:"entity_type"` // 实体类型：table/column/metric/relation
	EntityID   string                 `json:"entity_id"`   // 实体ID
	Score      float64                `json:"score"`       // 相似度分数
	Content    string                 `json:"content"`     // 文本内容
	Metadata   map[string]interface{} `json:"metadata"`    // 元数据
}

// SearchSchema 搜索Schema（向量检索）
func (s *NL2SQLVectorSearcher) SearchSchema(ctx context.Context, req *SearchSchemaRequest) (*SearchSchemaResponse, error) {
	// 设置默认值
	if req.TopK == 0 {
		req.TopK = 15
	}
	if req.MinScore == 0 {
		req.MinScore = 0.3
	}

	// 从数据库查询数据源信息，获取vector_database字段
	var ds dbgorm.NL2SQLDataSource
	if err := s.db.First(&ds, "id = ?", req.SchemaID).Error; err != nil {
		return nil, fmt.Errorf("数据源不存在: %w", err)
	}

	collectionName := ds.VectorDatabase
	if collectionName == "" {
		return nil, fmt.Errorf("数据源的vector_database字段为空")
	}

	g.Log().Infof(ctx, "NL2SQL向量搜索 - Collection: %s, Query: %s, TopK: %d, MinScore: %.2f",
		collectionName, req.Query, req.TopK, req.MinScore)

	// 检查collection是否存在
	exists, err := s.vectorStore.CollectionExists(ctx, collectionName)
	if err != nil {
		return nil, fmt.Errorf("检查collection失败: %w", err)
	}
	if !exists {
		g.Log().Warningf(ctx, "Collection不存在: %s，可能Schema尚未向量化", collectionName)
		return &SearchSchemaResponse{Results: []*SchemaSearchResult{}}, nil
	}

	// 执行向量搜索
	docs, err := s.vectorStore.VectorSearchOnly(
		ctx,
		&nl2sqlRetrieverConfig{
			topK:  req.TopK,
			score: req.MinScore,
		},
		req.Query,
		req.SchemaID, // 使用SchemaID作为knowledge_id进行过滤
		req.TopK,
		req.MinScore,
	)
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	g.Log().Infof(ctx, "向量搜索完成，返回 %d 个结果", len(docs))

	// 转换结果
	results := make([]*SchemaSearchResult, 0, len(docs))
	for _, doc := range docs {
		result := &SchemaSearchResult{
			DocumentID: doc.ID, // 使用ID作为DocumentID
			ChunkID:    doc.ID,
			Score:      float64(doc.Score), // 转换float32到float64
			Content:    doc.Content,
			Metadata:   doc.MetaData, // 使用MetaData字段
		}

		// 从metadata中提取entity信息
		if entityType, ok := doc.MetaData["entity_type"].(string); ok {
			result.EntityType = entityType
		}
		if entityID, ok := doc.MetaData["entity_id"].(string); ok {
			result.EntityID = entityID
		}

		results = append(results, result)
	}

	return &SearchSchemaResponse{
		Results: results,
	}, nil
}

// SearchSchemaSimple 简化版Schema搜索（直接返回VectorSearchResult格式）
func (s *NL2SQLVectorSearcher) SearchSchemaSimple(ctx context.Context, schemaID, query string, topK int) ([]VectorSearchResult, error) {
	req := &SearchSchemaRequest{
		SchemaID: schemaID,
		Query:    query,
		TopK:     topK,
		MinScore: 0.3,
	}

	resp, err := s.SearchSchema(ctx, req)
	if err != nil {
		return nil, err
	}

	// 转换为VectorSearchResult格式
	results := make([]VectorSearchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		results = append(results, VectorSearchResult{
			DocumentID: r.DocumentID,
			ChunkID:    r.ChunkID,
			Score:      r.Score,
			Content:    r.Content,
		})
	}

	return results, nil
}

// nl2sqlRetrieverConfig 实现GeneralRetrieverConfig接口
type nl2sqlRetrieverConfig struct {
	topK  int
	score float64
}

func (c *nl2sqlRetrieverConfig) GetTopK() int {
	return c.topK
}

func (c *nl2sqlRetrieverConfig) GetScore() float64 {
	return c.score
}

func (c *nl2sqlRetrieverConfig) GetEnableRewrite() bool {
	return false // NL2SQL不需要query重写
}

func (c *nl2sqlRetrieverConfig) GetRewriteAttempts() int {
	return 0
}

func (c *nl2sqlRetrieverConfig) GetRetrieveMode() string {
	return "simple" // NL2SQL使用简单向量检索，不需要Rerank
}

// ConvertDocsToVectorResults 将schema.Document转换为VectorSearchResult
func ConvertDocsToVectorResults(docs []*schema.Document) []VectorSearchResult {
	results := make([]VectorSearchResult, 0, len(docs))
	for _, doc := range docs {
		results = append(results, VectorSearchResult{
			DocumentID: doc.ID, // 使用ID作为DocumentID
			ChunkID:    doc.ID,
			Score:      float64(doc.Score), // 转换float32到float64
			Content:    doc.Content,
		})
	}
	return results
}
