package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/nl2sql/adapter"
	nl2sqlCommon "github.com/Malowking/kbgo/nl2sql/common"
	"github.com/Malowking/kbgo/nl2sql/datasource"
	"github.com/Malowking/kbgo/nl2sql/generator"
	"github.com/Malowking/kbgo/nl2sql/vector"
	"github.com/gogf/gf/v2/frame/g"
)

// QueryWithAdapters 执行NL2SQL查询（完整实现，使用适配器）
func (s *NL2SQLService) QueryWithAdapters(
	ctx context.Context,
	req *QueryRequest,
	llmAdapter *adapter.LLMAdapter,
	vectorAdapter *adapter.VectorSearchAdapter,
) (*QueryResponse, error) {
	g.Log().Infof(ctx, "QueryWithAdapters started - DatasourceID: %s, Question: %s", req.DatasourceID, req.Question)

	// 1. 获取数据源信息
	var ds dbgorm.NL2SQLDataSource
	if err := s.db.First(&ds, "id = ?", req.DatasourceID).Error; err != nil {
		return nil, fmt.Errorf("数据源不存在: %w", err)
	}

	// 2. 检查是否有已解析的表
	var parsedTableCount int64
	if err := s.db.Model(&dbgorm.NL2SQLTable{}).
		Where("datasource_id = ? AND parsed = ?", req.DatasourceID, true).
		Count(&parsedTableCount).Error; err != nil {
		return nil, fmt.Errorf("检查表状态失败: %w", err)
	}
	if parsedTableCount == 0 {
		return nil, fmt.Errorf("没有已解析的表，请先解析数据源")
	}

	// 3. 创建查询日志
	queryLog := &dbgorm.NL2SQLQueryLog{
		DatasourceID:    req.DatasourceID,
		UserQuestion:    req.Question,
		ExecutionStatus: "processing",
	}
	if err := s.db.Create(queryLog).Error; err != nil {
		return nil, fmt.Errorf("创建查询日志失败: %w", err)
	}

	g.Log().Infof(ctx, "Query log created: %s", queryLog.ID)

	// 4. Schema向量召回
	g.Log().Debug(ctx, "Starting schema retrieval...")
	retriever := vector.NewSchemaRetriever(s.db)

	retrieveReq := &vector.RetrieveRequest{
		SchemaID: req.DatasourceID, // 使用 DatasourceID 代替 SchemaID
		Query:    req.Question,
		TopK:     5,
		MinScore: 0.3,
	}

	// 定义向量搜索函数
	vectorSearchFunc := func(query string, topK int) ([]vector.VectorSearchResult, error) {
		if vectorAdapter == nil {
			// 如果没有向量搜索适配器，返回空结果（降级为全表查询）
			g.Log().Warning(ctx, "No vector adapter, will use all tables")
			return []vector.VectorSearchResult{}, nil
		}

		results, err := vectorAdapter.Search(ctx, query, topK)
		if err != nil {
			g.Log().Warningf(ctx, "Vector search failed: %v, falling back to all tables", err)
			return []vector.VectorSearchResult{}, nil
		}

		// 转换适配器结果为检索器需要的格式
		var vectorResults []vector.VectorSearchResult
		for _, r := range results {
			vectorResults = append(vectorResults, vector.VectorSearchResult{
				DocumentID: r.DocumentID,
				ChunkID:    r.ChunkID,
				Score:      r.Score,
			})
		}
		return vectorResults, nil
	}

	retrieveResult, err := retriever.Retrieve(ctx, retrieveReq, vectorSearchFunc)
	if err != nil {
		g.Log().Warningf(ctx, "Schema retrieval failed: %v, will use simplified approach", err)
		// 继续执行，但使用简化的Schema上下文
	}

	// 5. 构建Schema上下文
	g.Log().Debugf(ctx, "Building schema context - Tables: %d, Metrics: %d",
		len(retrieveResult.Tables), len(retrieveResult.Metrics))

	sqlGenerator := generator.NewSQLGenerator(s.db)
	var schemaContext *generator.SchemaContext

	if retrieveResult != nil && len(retrieveResult.Tables) > 0 {
		// 使用向量召回的表
		schemaContext = buildSchemaContextFromRetrieval(retrieveResult)
	} else {
		// 降级：使用所有表（限制前5个）
		g.Log().Warning(ctx, "Using fallback: loading all tables")
		schemaContext, err = sqlGenerator.RecallSchema(ctx, req.DatasourceID, req.Question)
		if err != nil {
			return nil, fmt.Errorf("构建Schema上下文失败: %w", err)
		}
	}

	// 6. 使用LLM生成SQL
	g.Log().Debug(ctx, "Generating SQL with LLM...")
	generateReq := &generator.GenerateRequest{
		Question:      req.Question,
		SchemaContext: schemaContext,
	}

	if llmAdapter == nil {
		return nil, fmt.Errorf("LLM适配器未配置")
	}

	generateResp, err := sqlGenerator.Generate(ctx, generateReq, func(prompt string) (string, error) {
		return llmAdapter.Call(ctx, prompt)
	})
	if err != nil {
		s.updateQueryLogStatus(queryLog.ID, nl2sqlCommon.ExecutionStatusFailed, "", fmt.Sprintf("SQL生成失败: %v", err))
		return nil, fmt.Errorf("SQL生成失败: %w", err)
	}

	generatedSQL := generateResp.SQL
	g.Log().Infof(ctx, "Generated SQL: %s", generatedSQL)

	// 7. SQL校验
	g.Log().Debug(ctx, "Validating SQL...")
	if err := s.sqlValidator.Validate(generatedSQL); err != nil {
		s.updateQueryLogStatus(queryLog.ID, nl2sqlCommon.ExecutionStatusFailed, generatedSQL, fmt.Sprintf("SQL校验失败: %v", err))
		return &QueryResponse{
			QueryLogID:  queryLog.ID,
			SQL:         generatedSQL,
			Explanation: generateResp.Reasoning,
			Error:       fmt.Sprintf("SQL校验失败: %v", err),
		}, nil
	}

	// 8. 执行SQL
	g.Log().Debug(ctx, "Executing SQL...")
	queryResult, err := s.executeSQL(ctx, &ds, generatedSQL)
	if err != nil {
		s.updateQueryLogStatus(queryLog.ID, nl2sqlCommon.ExecutionStatusFailed, generatedSQL, fmt.Sprintf("SQL执行失败: %v", err))
		return &QueryResponse{
			QueryLogID:  queryLog.ID,
			SQL:         generatedSQL,
			Explanation: generateResp.Reasoning,
			Error:       fmt.Sprintf("SQL执行失败: %v", err),
		}, nil
	}

	// 9. 更新查询日志
	resultJSON, _ := json.Marshal(queryResult)
	s.db.Model(queryLog).Updates(map[string]interface{}{
		"status":      nl2sqlCommon.ExecutionStatusSuccess,
		"sql":         generatedSQL,
		"result":      resultJSON,
		"explanation": generateResp.Reasoning,
	})

	g.Log().Infof(ctx, "Query completed successfully - RowCount: %d", queryResult.RowCount)

	// 创建返回响应
	queryResp := &QueryResponse{
		QueryLogID:  queryLog.ID,
		SQL:         generatedSQL,
		Result:      queryResult,
		Explanation: generateResp.Reasoning,
	}

	return queryResp, nil
}

// buildSchemaContextFromRetrieval 从向量召回结果构建Schema上下文
func buildSchemaContextFromRetrieval(retrieveResult *vector.RetrieveResult) *generator.SchemaContext {
	schemaContext := &generator.SchemaContext{
		Tables:    make([]generator.TableContext, 0),
		Metrics:   make([]generator.MetricContext, 0),
		Relations: make([]generator.RelationContext, 0),
	}

	// 转换表信息
	for _, table := range retrieveResult.Tables {
		tableCtx := generator.TableContext{
			Name:        table.TableName,
			DisplayName: table.DisplayName,
			Description: table.Description,
			Columns:     make([]generator.ColumnContext, 0),
		}

		for _, col := range table.Columns {
			tableCtx.Columns = append(tableCtx.Columns, generator.ColumnContext{
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
		schemaContext.Metrics = append(schemaContext.Metrics, generator.MetricContext{
			MetricID:    metric.MetricID,
			Name:        metric.Name,
			Description: metric.Description,
			Formula:     metric.Formula,
		})
	}

	// 转换关系信息
	for _, rel := range retrieveResult.Relations {
		schemaContext.Relations = append(schemaContext.Relations, generator.RelationContext{
			FromTable: rel.FromTable,
			FromCol:   rel.FromColumn,
			ToTable:   rel.ToTable,
			ToCol:     rel.ToColumn,
		})
	}

	return schemaContext
}

// executeSQL 执行SQL查询
func (s *NL2SQLService) executeSQL(ctx context.Context, ds *dbgorm.NL2SQLDataSource, sql string) (*QueryResult, error) {
	// 解析数据源配置
	var config map[string]interface{}
	if err := json.Unmarshal(ds.Config, &config); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 创建数据源连接
	dbConfig := mapToDBConfig(config, ds.DBType)
	connector := datasource.NewJDBCConnector(dbConfig)

	if err := connector.Connect(ctx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	defer connector.Close()

	// 执行查询
	result, err := connector.ExecuteQuery(ctx, sql)
	if err != nil {
		return nil, err
	}

	// 转换结果格式 (*datasource.QueryResult -> *QueryResult)
	if len(result.Rows) == 0 {
		return &QueryResult{
			Columns:  result.Columns,
			Data:     []map[string]interface{}{},
			RowCount: 0,
		}, nil
	}

	data := make([]map[string]interface{}, 0, len(result.Rows))
	for _, row := range result.Rows {
		rowMap := make(map[string]interface{})
		for i, colName := range result.Columns {
			if i < len(row) {
				rowMap[colName] = row[i]
			}
		}
		data = append(data, rowMap)
	}

	return &QueryResult{
		Columns:  result.Columns,
		Data:     data,
		RowCount: len(data),
	}, nil
}

// updateQueryLogStatus 更新查询日志状态
func (s *NL2SQLService) updateQueryLogStatus(queryLogID, status, sql, errorMsg string) {
	updates := map[string]interface{}{
		"status": status,
	}
	if sql != "" {
		updates["sql"] = sql
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	s.db.Model(&dbgorm.NL2SQLQueryLog{}).Where("id = ?", queryLogID).Updates(updates)
}

// SimplifiedQuery 简化版查询（不依赖向量搜索，用于测试）
func (s *NL2SQLService) SimplifiedQuery(
	ctx context.Context,
	req *QueryRequest,
	llmFunc func(prompt string) (string, error),
) (*QueryResponse, error) {
	g.Log().Infof(ctx, "SimplifiedQuery started - DatasourceID: %s", req.DatasourceID)

	// 1. 获取数据源
	var ds dbgorm.NL2SQLDataSource
	if err := s.db.First(&ds, "id = ?", req.DatasourceID).Error; err != nil {
		return nil, fmt.Errorf("数据源不存在: %w", err)
	}

	// 2. 检查是否有已解析的表
	var parsedTableCount int64
	if err := s.db.Model(&dbgorm.NL2SQLTable{}).
		Where("datasource_id = ? AND parsed = ?", req.DatasourceID, true).
		Count(&parsedTableCount).Error; err != nil {
		return nil, fmt.Errorf("检查表状态失败: %w", err)
	}
	if parsedTableCount == 0 {
		return nil, fmt.Errorf("没有已解析的表，请先解析数据源")
	}

	// 3. 创建查询日志
	queryLog := &dbgorm.NL2SQLQueryLog{
		DatasourceID:    req.DatasourceID,
		UserQuestion:    req.Question,
		ExecutionStatus: "processing",
	}
	if err := s.db.Create(queryLog).Error; err != nil {
		return nil, fmt.Errorf("创建查询日志失败: %w", err)
	}
	queryLogID := queryLog.ID

	// 4. 构建简化的Schema上下文（取前3个表）
	sqlGenerator := generator.NewSQLGenerator(s.db)
	schemaContext, err := sqlGenerator.RecallSchema(ctx, req.DatasourceID, req.Question)
	if err != nil {
		return nil, fmt.Errorf("构建Schema上下文失败: %w", err)
	}

	// 5. 生成SQL
	generateReq := &generator.GenerateRequest{
		Question:      req.Question,
		SchemaContext: schemaContext,
	}

	generateResp, err := sqlGenerator.Generate(ctx, generateReq, llmFunc)
	if err != nil {
		s.updateQueryLogStatus(queryLogID, nl2sqlCommon.ExecutionStatusFailed, "", err.Error())
		return nil, fmt.Errorf("SQL生成失败: %w", err)
	}

	// 6. 提取SQL（从可能的JSON或markdown格式中）
	generatedSQL := extractSQL(generateResp.SQL)
	g.Log().Infof(ctx, "Generated SQL: %s", generatedSQL)

	// 7. 校验SQL
	if err := s.sqlValidator.Validate(generatedSQL); err != nil {
		s.updateQueryLogStatus(queryLogID, nl2sqlCommon.ExecutionStatusFailed, generatedSQL, err.Error())
		return &QueryResponse{
			QueryLogID:  queryLogID,
			SQL:         generatedSQL,
			Explanation: generateResp.Reasoning,
			Error:       fmt.Sprintf("SQL校验失败: %v", err),
		}, nil
	}

	// 8. 执行SQL
	queryResult, err := s.executeSQL(ctx, &ds, generatedSQL)
	if err != nil {
		s.updateQueryLogStatus(queryLogID, nl2sqlCommon.ExecutionStatusFailed, generatedSQL, err.Error())
		return &QueryResponse{
			QueryLogID:  queryLogID,
			SQL:         generatedSQL,
			Explanation: generateResp.Reasoning,
			Error:       fmt.Sprintf("SQL执行失败: %v", err),
		}, nil
	}

	// 9. 更新日志
	resultJSON, _ := json.Marshal(queryResult)
	s.db.Model(queryLog).Updates(map[string]interface{}{
		"status":      nl2sqlCommon.ExecutionStatusSuccess,
		"sql":         generatedSQL,
		"result":      resultJSON,
		"explanation": generateResp.Reasoning,
	})

	return &QueryResponse{
		QueryLogID:  queryLogID,
		SQL:         generatedSQL,
		Result:      queryResult,
		Explanation: generateResp.Reasoning,
	}, nil
}

// extractSQL 从LLM响应中提取SQL（处理JSON或markdown格式）
func extractSQL(response string) string {
	// 尝试从```sql代码块中提取
	if strings.Contains(response, "```sql") {
		start := strings.Index(response, "```sql") + 6
		end := strings.Index(response[start:], "```")
		if end > 0 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	// 尝试从JSON中提取
	if strings.Contains(response, `"sql"`) {
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(response), &result); err == nil {
			if sql, ok := result["sql"].(string); ok {
				return strings.TrimSpace(sql)
			}
		}
	}

	// 直接返回（假设已经是纯SQL）
	return strings.TrimSpace(response)
}
