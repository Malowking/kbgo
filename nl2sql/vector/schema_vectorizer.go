package vector

import (
	"context"
	"encoding/json"
	"fmt"

	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SchemaVectorizer Schema向量化器
type SchemaVectorizer struct {
	db *gorm.DB
}

// NewSchemaVectorizer 创建Schema向量化器
func NewSchemaVectorizer(db *gorm.DB) *SchemaVectorizer {
	return &SchemaVectorizer{
		db: db,
	}
}

// VectorizeSchemaRequest 向量化请求
type VectorizeSchemaRequest struct {
	DatasourceID    string                               `json:"datasource_id"`
	KnowledgeBaseID string                               `json:"knowledge_base_id"`
	EmbeddingFunc   func(text string) ([]float32, error) // 向量化函数
	StoreFunc       func(doc *VectorDocument) error      // 存储函数
}

// VectorDocument 向量文档
type VectorDocument struct {
	DocumentID string                 `json:"document_id"`
	ChunkID    string                 `json:"chunk_id"`
	Content    string                 `json:"content"`
	Vector     []float32              `json:"vector"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// VectorizeSchema 向量化整个Schema
func (v *SchemaVectorizer) VectorizeSchema(ctx context.Context, req *VectorizeSchemaRequest) error {
	g.Log().Infof(ctx, "Starting schema vectorization - DatasourceID: %s, KBID: %s", req.DatasourceID, req.KnowledgeBaseID)

	// 1. 验证数据源存在
	var datasource dbgorm.NL2SQLDataSource
	if err := v.db.First(&datasource, "id = ?", req.DatasourceID).Error; err != nil {
		return fmt.Errorf("获取数据源失败: %w", err)
	}

	// 2. 向量化数据源概览（作为顶层 Document）
	if err := v.vectorizeDatasource(ctx, req, &datasource); err != nil {
		g.Log().Warningf(ctx, "向量化数据源概览失败（非致命）: %v", err)
		// 数据源概览失败不影响整体流程
	}

	// 3. 向量化表（Table）
	if err := v.vectorizeTables(ctx, req); err != nil {
		return fmt.Errorf("向量化表失败: %w", err)
	}

	// 4. 向量化列（Column）
	if err := v.vectorizeColumns(ctx, req); err != nil {
		return fmt.Errorf("向量化列失败: %w", err)
	}

	// 5. 向量化指标（Metric）
	if err := v.vectorizeMetrics(ctx, req); err != nil {
		return fmt.Errorf("向量化指标失败: %w", err)
	}

	// 6. 向量化关系（Relation）
	if err := v.vectorizeRelations(ctx, req); err != nil {
		return fmt.Errorf("向量化关系失败: %w", err)
	}

	g.Log().Infof(ctx, "Schema vectorization completed - DatasourceID: %s", req.DatasourceID)
	return nil
}

// vectorizeTables 向量化表
func (v *SchemaVectorizer) vectorizeTables(ctx context.Context, req *VectorizeSchemaRequest) error {
	var tables []dbgorm.NL2SQLTable
	if err := v.db.Where("datasource_id = ?", req.DatasourceID).Find(&tables).Error; err != nil {
		return err
	}

	g.Log().Infof(ctx, "Vectorizing %d tables", len(tables))

	for _, table := range tables {
		// 构建文本内容
		content := v.buildTableContent(ctx, &table)

		// 生成向量
		vector, err := req.EmbeddingFunc(content)
		if err != nil {
			g.Log().Warningf(ctx, "Failed to embed table %s: %v", table.Name, err)
			continue
		}

		// 使用表的ID作为document_id（表本身是一个文档）
		doc := &VectorDocument{
			DocumentID: table.ID, // 使用表ID作为document_id
			ChunkID:    uuid.New().String(),
			Content:    content,
			Vector:     vector,
			Metadata: map[string]interface{}{
				"datasource_id": req.DatasourceID,
				"entity_type":   "table",
				"entity_id":     table.ID,
				"table_id":      table.ID,
				"table_name":    table.Name,
			},
		}

		// 存储向量
		if err := req.StoreFunc(doc); err != nil {
			g.Log().Warningf(ctx, "Failed to store table vector %s: %v", table.Name, err)
			continue
		}

		// 记录向量文档关联
		vectorDoc := &dbgorm.NL2SQLVectorDoc{
			DatasourceID:  req.DatasourceID,
			EntityType:    "table",
			EntityID:      table.ID,
			DocumentID:    table.ID, // document_id = table.ID
			VectorContent: content,
		}
		if err := v.db.Create(vectorDoc).Error; err != nil {
			g.Log().Warningf(ctx, "Failed to create vector doc record for table %s: %v", table.Name, err)
		}
	}

	return nil
}

// vectorizeColumns 向量化列
func (v *SchemaVectorizer) vectorizeColumns(ctx context.Context, req *VectorizeSchemaRequest) error {
	// 获取所有表
	var tables []dbgorm.NL2SQLTable
	if err := v.db.Where("datasource_id = ?", req.DatasourceID).Find(&tables).Error; err != nil {
		return err
	}

	totalColumns := 0
	for _, table := range tables {
		// 获取表的列
		var columns []dbgorm.NL2SQLColumn
		if err := v.db.Where("table_id = ?", table.ID).Find(&columns).Error; err != nil {
			continue
		}

		totalColumns += len(columns)

		for _, column := range columns {
			// 构建文本内容
			content := v.buildColumnContent(&table, &column)

			// 生成向量
			vector, err := req.EmbeddingFunc(content)
			if err != nil {
				g.Log().Warningf(ctx, "Failed to embed column %s.%s: %v", table.Name, column.ColumnName, err)
				continue
			}

			// 列使用所属表的ID作为document_id（列是表文档的分块）
			doc := &VectorDocument{
				DocumentID: table.ID, // 使用表ID作为document_id
				ChunkID:    uuid.New().String(),
				Content:    content,
				Vector:     vector,
				Metadata: map[string]interface{}{
					"datasource_id": req.DatasourceID,
					"entity_type":   "column",
					"entity_id":     column.ID,
					"table_id":      table.ID,
					"table_name":    table.Name,
					"column_name":   column.ColumnName,
				},
			}

			// 存储向量
			if err := req.StoreFunc(doc); err != nil {
				g.Log().Warningf(ctx, "Failed to store column vector %s.%s: %v", table.Name, column.ColumnName, err)
				continue
			}

			// 记录向量文档关联
			vectorDoc := &dbgorm.NL2SQLVectorDoc{
				DatasourceID:  req.DatasourceID,
				EntityType:    "column",
				EntityID:      column.ID,
				DocumentID:    table.ID, // document_id = table.ID
				VectorContent: content,
			}
			if err := v.db.Create(vectorDoc).Error; err != nil {
				g.Log().Warningf(ctx, "Failed to create vector doc record for column %s.%s: %v", table.Name, column.ColumnName, err)
			}
		}
	}

	g.Log().Infof(ctx, "Vectorized %d columns", totalColumns)
	return nil
}

// vectorizeMetrics 向量化指标
func (v *SchemaVectorizer) vectorizeMetrics(ctx context.Context, req *VectorizeSchemaRequest) error {
	var metrics []dbgorm.NL2SQLMetric
	if err := v.db.Where("datasource_id = ?", req.DatasourceID).Find(&metrics).Error; err != nil {
		return err
	}

	g.Log().Infof(ctx, "Vectorizing %d metrics", len(metrics))

	for _, metric := range metrics {
		// 构建文本内容
		content := v.buildMetricContent(&metric)

		// 生成向量
		vector, err := req.EmbeddingFunc(content)
		if err != nil {
			g.Log().Warningf(ctx, "Failed to embed metric %s: %v", metric.Name, err)
			continue
		}

		// 创建文档
		documentID := uuid.New().String()
		doc := &VectorDocument{
			DocumentID: documentID,
			ChunkID:    uuid.New().String(),
			Content:    content,
			Vector:     vector,
			Metadata: map[string]interface{}{
				"datasource_id": req.DatasourceID,
				"entity_type":   "metric",
				"entity_id":     metric.ID,
				"metric_name":   metric.Name,
			},
		}

		// 存储向量
		if err := req.StoreFunc(doc); err != nil {
			g.Log().Warningf(ctx, "Failed to store metric vector %s: %v", metric.Name, err)
			continue
		}

		// 记录向量文档关联
		vectorDoc := &dbgorm.NL2SQLVectorDoc{
			DatasourceID:  req.DatasourceID,
			EntityType:    "metric",
			EntityID:      metric.ID,
			DocumentID:    documentID,
			VectorContent: content,
		}
		if err := v.db.Create(vectorDoc).Error; err != nil {
			g.Log().Warningf(ctx, "Failed to create vector doc record for metric %s: %v", metric.Name, err)
		}
	}

	return nil
}

// vectorizeRelations 向量化关系
func (v *SchemaVectorizer) vectorizeRelations(ctx context.Context, req *VectorizeSchemaRequest) error {
	var relations []dbgorm.NL2SQLRelation
	if err := v.db.Where("datasource_id = ?", req.DatasourceID).Find(&relations).Error; err != nil {
		return err
	}

	g.Log().Infof(ctx, "Vectorizing %d relations", len(relations))

	for _, relation := range relations {
		// 构建文本内容
		content := v.buildRelationContent(ctx, &relation)

		// 生成向量
		vector, err := req.EmbeddingFunc(content)
		if err != nil {
			g.Log().Warningf(ctx, "Failed to embed relation %s: %v", relation.RelationID, err)
			continue
		}

		// 创建文档
		documentID := uuid.New().String()
		doc := &VectorDocument{
			DocumentID: documentID,
			ChunkID:    uuid.New().String(),
			Content:    content,
			Vector:     vector,
			Metadata: map[string]interface{}{
				"datasource_id": req.DatasourceID,
				"entity_type":   "relation",
				"entity_id":     relation.ID,
			},
		}

		// 存储向量
		if err := req.StoreFunc(doc); err != nil {
			g.Log().Warningf(ctx, "Failed to store relation vector %s: %v", relation.RelationID, err)
			continue
		}

		// 记录向量文档关联
		vectorDoc := &dbgorm.NL2SQLVectorDoc{
			DatasourceID:  req.DatasourceID,
			EntityType:    "relation",
			EntityID:      relation.ID,
			DocumentID:    documentID,
			VectorContent: content,
		}
		if err := v.db.Create(vectorDoc).Error; err != nil {
			g.Log().Warningf(ctx, "Failed to create vector doc record for relation %s: %v", relation.RelationID, err)
		}
	}

	return nil
}

// buildTableContent 构建表的文本内容
func (v *SchemaVectorizer) buildTableContent(ctx context.Context, table *dbgorm.NL2SQLTable) string {
	// 获取表的列
	var columns []dbgorm.NL2SQLColumn
	v.db.Where("table_id = ?", table.ID).Find(&columns)

	content := fmt.Sprintf("表名: %s\n", table.Name)
	if table.DisplayName != "" && table.DisplayName != table.Name {
		content += fmt.Sprintf("显示名称: %s\n", table.DisplayName)
	}
	if table.Description != "" {
		content += fmt.Sprintf("描述: %s\n", table.Description)
	}

	content += "字段列表:\n"
	for _, col := range columns {
		content += fmt.Sprintf("  - %s (%s)", col.ColumnName, col.DataType)
		if col.Description != "" {
			content += fmt.Sprintf(": %s", col.Description)
		}
		content += "\n"
	}

	return content
}

// buildColumnContent 构建列的文本内容
func (v *SchemaVectorizer) buildColumnContent(table *dbgorm.NL2SQLTable, column *dbgorm.NL2SQLColumn) string {
	content := fmt.Sprintf("表名: %s\n", table.Name)
	content += fmt.Sprintf("列名: %s\n", column.ColumnName)
	if column.DisplayName != "" && column.DisplayName != column.ColumnName {
		content += fmt.Sprintf("显示名称: %s\n", column.DisplayName)
	}
	content += fmt.Sprintf("数据类型: %s\n", column.DataType)
	if column.Description != "" {
		content += fmt.Sprintf("描述: %s\n", column.Description)
	}
	if column.SemanticType != "" {
		content += fmt.Sprintf("语义类型: %s\n", column.SemanticType)
	}

	// 添加示例值
	if column.Examples != nil && len(column.Examples) > 0 {
		var examples []string
		json.Unmarshal(column.Examples, &examples)
		if len(examples) > 0 {
			content += fmt.Sprintf("示例值: %v\n", examples)
		}
	}

	return content
}

// buildMetricContent 构建指标的文本内容
func (v *SchemaVectorizer) buildMetricContent(metric *dbgorm.NL2SQLMetric) string {
	content := fmt.Sprintf("指标名称: %s\n", metric.Name)
	if metric.Description != "" {
		content += fmt.Sprintf("描述: %s\n", metric.Description)
	}
	if metric.Formula != "" {
		content += fmt.Sprintf("计算公式: %s\n", metric.Formula)
	}

	return content
}

// buildRelationContent 构建关系的文本内容
func (v *SchemaVectorizer) buildRelationContent(ctx context.Context, relation *dbgorm.NL2SQLRelation) string {
	// 获取表名
	var fromTable, toTable dbgorm.NL2SQLTable
	v.db.First(&fromTable, "id = ?", relation.FromTableID)
	v.db.First(&toTable, "id = ?", relation.ToTableID)

	content := fmt.Sprintf("关系: %s.%s -> %s.%s\n",
		fromTable.Name, relation.FromColumn,
		toTable.Name, relation.ToColumn)
	content += fmt.Sprintf("关系类型: %s\n", relation.RelationType)
	if relation.Description != "" {
		content += fmt.Sprintf("描述: %s\n", relation.Description)
	}

	return content
}

// vectorizeDatasource 向量化数据源概览
func (v *SchemaVectorizer) vectorizeDatasource(ctx context.Context, req *VectorizeSchemaRequest, datasource *dbgorm.NL2SQLDataSource) error {
	// 获取所有表名，构建概览
	var tables []dbgorm.NL2SQLTable
	if err := v.db.Where("datasource_id = ?", req.DatasourceID).Find(&tables).Error; err != nil {
		return err
	}

	tableNames := make([]string, len(tables))
	for i, t := range tables {
		tableNames[i] = t.Name
	}

	// 构建数据源概览内容
	content := v.buildDatasourceContent(datasource, tableNames)

	// 生成向量
	vector, err := req.EmbeddingFunc(content)
	if err != nil {
		return fmt.Errorf("向量化数据源失败: %w", err)
	}

	// 使用数据源ID作为document_id
	doc := &VectorDocument{
		DocumentID: datasource.ID,
		ChunkID:    uuid.New().String(),
		Content:    content,
		Vector:     vector,
		Metadata: map[string]interface{}{
			"datasource_id":   req.DatasourceID,
			"entity_type":     "datasource",
			"entity_id":       datasource.ID,
			"datasource_name": datasource.Name,
			"datasource_type": datasource.Type,
		},
	}

	// 存储向量
	if err := req.StoreFunc(doc); err != nil {
		return fmt.Errorf("存储数据源向量失败: %w", err)
	}

	// 记录向量文档关联
	vectorDoc := &dbgorm.NL2SQLVectorDoc{
		DatasourceID:  req.DatasourceID,
		EntityType:    "datasource",
		EntityID:      datasource.ID,
		DocumentID:    datasource.ID,
		VectorContent: content,
	}
	if err := v.db.Create(vectorDoc).Error; err != nil {
		g.Log().Warningf(ctx, "Failed to create vector doc record for datasource: %v", err)
	}

	g.Log().Infof(ctx, "Datasource overview vectorized: %s", datasource.Name)
	return nil
}

// buildDatasourceContent 构建数据源概览内容
func (v *SchemaVectorizer) buildDatasourceContent(datasource *dbgorm.NL2SQLDataSource, tableNames []string) string {
	content := fmt.Sprintf("数据源名称: %s\n", datasource.Name)
	content += fmt.Sprintf("数据源类型: %s\n", datasource.Type)

	if datasource.DBType != "" {
		content += fmt.Sprintf("数据库类型: %s\n", datasource.DBType)
	}

	// 未来可以添加业务领域、描述等字段
	// if datasource.Description != "" {
	//     content += fmt.Sprintf("描述: %s\n", datasource.Description)
	// }

	content += fmt.Sprintf("包含表数量: %d\n", len(tableNames))

	if len(tableNames) > 0 {
		content += "表列表:\n"
		for _, name := range tableNames {
			content += fmt.Sprintf("  - %s\n", name)
		}
	}

	return content
}
