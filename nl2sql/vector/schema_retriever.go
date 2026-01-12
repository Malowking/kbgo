package vector

import (
	"context"
	"fmt"

	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"gorm.io/gorm"
)

// SchemaRetriever Schema向量检索器
type SchemaRetriever struct {
	db *gorm.DB
}

// NewSchemaRetriever 创建Schema检索器
func NewSchemaRetriever(db *gorm.DB) *SchemaRetriever {
	return &SchemaRetriever{
		db: db,
	}
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	SchemaID string  `json:"schema_id"`
	Query    string  `json:"query"`
	TopK     int     `json:"top_k"`     // 默认5
	MinScore float64 `json:"min_score"` // 默认0.5
}

// RetrieveResult 检索结果
type RetrieveResult struct {
	Tables    []TableResult    `json:"tables"`
	Metrics   []MetricResult   `json:"metrics"`
	Relations []RelationResult `json:"relations"`
}

// TableResult 表检索结果
type TableResult struct {
	TableID     string         `json:"table_id"`
	TableName   string         `json:"table_name"`
	DisplayName string         `json:"display_name"`
	Description string         `json:"description"`
	Columns     []ColumnResult `json:"columns"`
	Score       float64        `json:"score"`
}

// ColumnResult 字段检索结果
type ColumnResult struct {
	ColumnID    string  `json:"column_id"`
	ColumnName  string  `json:"column_name"`
	DisplayName string  `json:"display_name"`
	DataType    string  `json:"data_type"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
}

// MetricResult 指标检索结果
type MetricResult struct {
	MetricCode  string  `json:"metric_code"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Formula     string  `json:"formula"`
	Score       float64 `json:"score"`
}

// RelationResult 关系检索结果
type RelationResult struct {
	RelationID   string  `json:"relation_id"`
	FromTable    string  `json:"from_table"`
	FromColumn   string  `json:"from_column"`
	ToTable      string  `json:"to_table"`
	ToColumn     string  `json:"to_column"`
	RelationType string  `json:"relation_type"`
	Score        float64 `json:"score"`
}

// Retrieve 执行向量检索
func (r *SchemaRetriever) Retrieve(ctx context.Context, req *RetrieveRequest, vectorSearchFunc func(query string, topK int) ([]VectorSearchResult, error)) (*RetrieveResult, error) {
	if req.TopK == 0 {
		req.TopK = 5
	}
	if req.MinScore == 0 {
		req.MinScore = 0.5
	}

	// 1. 调用向量搜索（复用现有的向量搜索能力）
	searchResults, err := vectorSearchFunc(req.Query, req.TopK*3) // 多召回一些候选
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	// 2. 解析向量搜索结果
	result := &RetrieveResult{
		Tables:    make([]TableResult, 0),
		Metrics:   make([]MetricResult, 0),
		Relations: make([]RelationResult, 0),
	}

	// 记录已召回的实体（去重）
	retrievedTables := make(map[string]bool)
	retrievedMetrics := make(map[string]bool)

	for _, searchResult := range searchResults {
		if searchResult.Score < req.MinScore {
			continue
		}

		// 通过document_id查找对应的nl2sql实体
		var vectorDocs []dbgorm.NL2SQLVectorDoc
		if err := r.db.Where("document_id = ? AND datasource_id = ?", searchResult.DocumentID, req.SchemaID).
			Find(&vectorDocs).Error; err != nil {
			continue
		}

		for _, vectorDoc := range vectorDocs {
			switch vectorDoc.EntityType {
			case "table":
				if !retrievedTables[vectorDoc.EntityID] {
					tableResult, err := r.retrieveTable(ctx, vectorDoc.EntityID, searchResult.Score)
					if err == nil {
						result.Tables = append(result.Tables, *tableResult)
						retrievedTables[vectorDoc.EntityID] = true
					}
				}

			case "metric":
				if !retrievedMetrics[vectorDoc.EntityID] {
					metricResult, err := r.retrieveMetric(ctx, vectorDoc.EntityID, searchResult.Score)
					if err == nil {
						result.Metrics = append(result.Metrics, *metricResult)
						retrievedMetrics[vectorDoc.EntityID] = true
					}
				}

			case "relation":
				relationResult, err := r.retrieveRelation(ctx, vectorDoc.EntityID, searchResult.Score)
				if err == nil {
					result.Relations = append(result.Relations, *relationResult)
				}
			}
		}
	}

	// 3. 关系传播：召回表的相关表
	if len(result.Tables) > 0 {
		relatedTables := r.propagateRelations(ctx, result.Tables, retrievedTables)
		result.Tables = append(result.Tables, relatedTables...)
	}

	// 4. 限制返回数量
	if len(result.Tables) > req.TopK {
		result.Tables = result.Tables[:req.TopK]
	}
	if len(result.Metrics) > req.TopK {
		result.Metrics = result.Metrics[:req.TopK]
	}

	return result, nil
}

// retrieveTable 检索表详情
func (r *SchemaRetriever) retrieveTable(ctx context.Context, tableID string, score float64) (*TableResult, error) {
	var table dbgorm.NL2SQLTable
	if err := r.db.First(&table, "id = ?", tableID).Error; err != nil {
		return nil, err
	}

	// 获取字段
	var columns []dbgorm.NL2SQLColumn
	r.db.Where("table_id = ?", tableID).Find(&columns)

	tableResult := &TableResult{
		TableID:     table.ID,
		TableName:   table.Name,
		DisplayName: table.DisplayName,
		Description: table.Description,
		Score:       score,
		Columns:     make([]ColumnResult, 0),
	}

	for _, col := range columns {
		tableResult.Columns = append(tableResult.Columns, ColumnResult{
			ColumnID:    col.ID,
			ColumnName:  col.ColumnName,
			DisplayName: col.DisplayName,
			DataType:    col.DataType,
			Description: col.Description,
			Score:       score,
		})
	}

	return tableResult, nil
}

// retrieveMetric 检索指标详情
func (r *SchemaRetriever) retrieveMetric(ctx context.Context, metricID string, score float64) (*MetricResult, error) {
	var metric dbgorm.NL2SQLMetric
	if err := r.db.First(&metric, "id = ?", metricID).Error; err != nil {
		return nil, err
	}

	return &MetricResult{
		MetricCode:  metric.MetricCode,
		Name:        metric.Name,
		Description: metric.Description,
		Formula:     metric.Formula,
		Score:       score,
	}, nil
}

// retrieveRelation 检索关系详情
func (r *SchemaRetriever) retrieveRelation(ctx context.Context, relationID string, score float64) (*RelationResult, error) {
	var relation dbgorm.NL2SQLRelation
	if err := r.db.First(&relation, "id = ?", relationID).Error; err != nil {
		return nil, err
	}

	// 获取表名
	var fromTable, toTable dbgorm.NL2SQLTable
	r.db.First(&fromTable, "id = ?", relation.FromTableID)
	r.db.First(&toTable, "id = ?", relation.ToTableID)

	return &RelationResult{
		RelationID:   relation.ID,
		FromTable:    fromTable.Name,
		FromColumn:   relation.FromColumn,
		ToTable:      toTable.Name,
		ToColumn:     relation.ToColumn,
		RelationType: relation.RelationType,
		Score:        score,
	}, nil
}

// propagateRelations 关系传播：基于已召回的表，召回相关联的表
func (r *SchemaRetriever) propagateRelations(ctx context.Context, tables []TableResult, retrieved map[string]bool) []TableResult {
	relatedTables := make([]TableResult, 0)

	for _, table := range tables {
		// 查找与该表相关的关系
		var relations []dbgorm.NL2SQLRelation
		r.db.Where("from_table_id = ? OR to_table_id = ?", table.TableID, table.TableID).
			Limit(3). // 每个表最多传播3个相关表
			Find(&relations)

		for _, rel := range relations {
			// 确定相关表ID
			relatedTableID := rel.ToTableID
			if relatedTableID == table.TableID {
				relatedTableID = rel.FromTableID
			}

			// 如果未召回，则添加
			if !retrieved[relatedTableID] {
				relatedTable, err := r.retrieveTable(ctx, relatedTableID, table.Score*0.8) // 传播的分数略低
				if err == nil {
					relatedTables = append(relatedTables, *relatedTable)
					retrieved[relatedTableID] = true
				}
			}
		}
	}

	return relatedTables
}

// VectorSearchResult 向量搜索结果（接口定义）
type VectorSearchResult struct {
	DocumentID string  `json:"document_id"`
	ChunkID    string  `json:"chunk_id"`
	Score      float64 `json:"score"`
	Content    string  `json:"content"`
}

// GetRelatedTables 获取表的直接关联表（不经过向量搜索）
func (r *SchemaRetriever) GetRelatedTables(ctx context.Context, tableID string) ([]string, error) {
	var relations []dbgorm.NL2SQLRelation
	if err := r.db.Where("from_table_id = ? OR to_table_id = ?", tableID, tableID).
		Find(&relations).Error; err != nil {
		return nil, err
	}

	relatedTableIDs := make([]string, 0)
	seen := make(map[string]bool)

	for _, rel := range relations {
		relatedID := rel.ToTableID
		if relatedID == tableID {
			relatedID = rel.FromTableID
		}

		if !seen[relatedID] {
			relatedTableIDs = append(relatedTableIDs, relatedID)
			seen[relatedID] = true
		}
	}

	return relatedTableIDs, nil
}
