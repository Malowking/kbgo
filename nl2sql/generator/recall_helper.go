package generator

import (
	"context"
	"fmt"

	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/nl2sql/vector"
	"github.com/gogf/gf/v2/frame/g"
)

// recallSchemaWithVector 使用向量检索召回Schema
func (gen *SQLGenerator) recallSchemaWithVector(ctx context.Context, schemaID, question string) (*SchemaContext, error) {
	g.Log().Infof(ctx, "开始向量检索 - SchemaID: %s, Question: %s", schemaID, question)

	// 创建向量搜索器（传入数据库连接）
	vectorSearcher, err := vector.NewNL2SQLVectorSearcher(gen.db)
	if err != nil {
		return nil, fmt.Errorf("创建向量搜索器失败: %w", err)
	}

	// 执行向量搜索
	vectorResults, err := vectorSearcher.SearchSchemaSimple(ctx, schemaID, question, 20)
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	if len(vectorResults) == 0 {
		g.Log().Warning(ctx, "向量搜索无结果")
		return &SchemaContext{
			Tables:    make([]TableContext, 0),
			Metrics:   make([]MetricContext, 0),
			Relations: make([]RelationContext, 0),
		}, nil
	}

	g.Log().Infof(ctx, "向量搜索返回 %d 个结果", len(vectorResults))

	// 使用SchemaRetriever解析向量搜索结果
	retriever := vector.NewSchemaRetriever(gen.db)
	retrieveReq := &vector.RetrieveRequest{
		SchemaID: schemaID,
		Query:    question,
		TopK:     5,
		MinScore: 0.3,
	}

	// 定义向量搜索函数（使用已有的搜索结果）
	vectorSearchFunc := func(query string, topK int) ([]vector.VectorSearchResult, error) {
		return vectorResults, nil
	}

	retrieveResult, err := retriever.Retrieve(ctx, retrieveReq, vectorSearchFunc)
	if err != nil {
		return nil, fmt.Errorf("Schema检索失败: %w", err)
	}

	g.Log().Infof(ctx, "Schema检索完成 - Tables: %d, Metrics: %d, Relations: %d",
		len(retrieveResult.Tables), len(retrieveResult.Metrics), len(retrieveResult.Relations))

	// 转换为SchemaContext
	return buildSchemaContextFromRetrieval(retrieveResult), nil
}

// recallSchemaFallback 降级实现：返回所有表（限制数量）
func (gen *SQLGenerator) recallSchemaFallback(ctx context.Context, schemaID string) (*SchemaContext, error) {
	g.Log().Info(ctx, "使用降级方案：查询所有表")

	var tables []dbgorm.NL2SQLTable
	if err := gen.db.Where("datasource_id = ?", schemaID).Limit(5).Find(&tables).Error; err != nil {
		return nil, err
	}

	schemaCtx := &SchemaContext{
		Tables:    make([]TableContext, 0),
		Metrics:   make([]MetricContext, 0),
		Relations: make([]RelationContext, 0),
	}

	// 构建表上下文
	for _, table := range tables {
		tableCtx := TableContext{
			Name:        fmt.Sprintf("nl2sql.%s", table.Name), // 添加schema前缀
			DisplayName: table.DisplayName,
			Description: table.Description,
			Columns:     make([]ColumnContext, 0),
		}

		// 获取列信息
		var columns []dbgorm.NL2SQLColumn
		if err := gen.db.Where("table_id = ?", table.ID).Find(&columns).Error; err != nil {
			continue
		}

		for _, col := range columns {
			tableCtx.Columns = append(tableCtx.Columns, ColumnContext{
				ColumnName:  col.ColumnName,
				DisplayName: col.DisplayName,
				DataType:    col.DataType,
				Description: col.Description,
			})
		}

		schemaCtx.Tables = append(schemaCtx.Tables, tableCtx)
	}

	// 获取关系
	var relations []dbgorm.NL2SQLRelation
	if err := gen.db.Where("datasource_id = ?", schemaID).Find(&relations).Error; err == nil {
		for _, rel := range relations {
			// 获取表名
			var fromTable, toTable dbgorm.NL2SQLTable
			gen.db.First(&fromTable, "id = ?", rel.FromTableID)
			gen.db.First(&toTable, "id = ?", rel.ToTableID)

			schemaCtx.Relations = append(schemaCtx.Relations, RelationContext{
				FromTable: fromTable.Name,
				FromCol:   rel.FromColumn,
				ToTable:   toTable.Name,
				ToCol:     rel.ToColumn,
			})
		}
	}

	g.Log().Infof(ctx, "降级方案完成 - Tables: %d, Relations: %d", len(schemaCtx.Tables), len(schemaCtx.Relations))

	return schemaCtx, nil
}

// buildSchemaContextFromRetrieval 从向量召回结果构建Schema上下文
func buildSchemaContextFromRetrieval(retrieveResult *vector.RetrieveResult) *SchemaContext {
	schemaContext := &SchemaContext{
		Tables:    make([]TableContext, 0),
		Metrics:   make([]MetricContext, 0),
		Relations: make([]RelationContext, 0),
	}

	// 转换表信息
	for _, table := range retrieveResult.Tables {
		tableCtx := TableContext{
			Name:        fmt.Sprintf("nl2sql.%s", table.TableName),
			DisplayName: table.DisplayName,
			Description: table.Description,
			Columns:     make([]ColumnContext, 0),
		}

		for _, col := range table.Columns {
			tableCtx.Columns = append(tableCtx.Columns, ColumnContext{
				ColumnName:  col.ColumnName,
				DisplayName: col.DisplayName,
				DataType:    col.DataType,
				Description: col.Description,
			})
		}

		schemaContext.Tables = append(schemaContext.Tables, tableCtx)
	}

	// 转换指标信息
	for _, metric := range retrieveResult.Metrics {
		schemaContext.Metrics = append(schemaContext.Metrics, MetricContext{
			MetricID:    metric.MetricID,
			Name:        metric.Name,
			Description: metric.Description,
			Formula:     metric.Formula,
		})
	}

	// 转换关系信息
	for _, rel := range retrieveResult.Relations {
		schemaContext.Relations = append(schemaContext.Relations, RelationContext{
			FromTable: rel.FromTable,
			FromCol:   rel.FromColumn,
			ToTable:   rel.ToTable,
			ToCol:     rel.ToColumn,
		})
	}

	return schemaContext
}
