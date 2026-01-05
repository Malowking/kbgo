package nl2sql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Malowking/kbgo/core/agent_tools/file_export"
	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/dao"
	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/internal/service"
	"github.com/Malowking/kbgo/nl2sql/adapter"
	nl2sqlservice "github.com/Malowking/kbgo/nl2sql/service"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// NL2SQLTool NL2SQL工具，用于在Chat中集成NL2SQL功能
type NL2SQLTool struct {
	nl2sqlService *nl2sqlservice.NL2SQLService
}

// NewNL2SQLTool 创建NL2SQL工具实例
func NewNL2SQLTool() *NL2SQLTool {
	db := dao.GetDB()
	redisClient := cache.GetRedisClient()
	nl2sqlSvc := nl2sqlservice.NewNL2SQLService(db, redisClient)

	return &NL2SQLTool{
		nl2sqlService: nl2sqlSvc,
	}
}

// UserIntent 用户意图
type UserIntent struct {
	IntentType    string   `json:"intent_type"`    // "data_only" | "need_analysis" | "need_visualization"
	NeedSQLOnly   bool     `json:"need_sql_only"`  // 是否只需要SQL，不执行
	NeedExplain   bool     `json:"need_explain"`   // 是否需要解释结果的含义
	DataLimit     int      `json:"data_limit"`     // 建议返回的数据行数限制
	AnalysisFocus []string `json:"analysis_focus"` // 用户关注的分析点
}

// NL2SQLResult NL2SQL执行结果
type NL2SQLResult struct {
	IsNL2SQLQuery   bool                     // 是否是NL2SQL查询
	QueryLogID      string                   // 查询日志ID
	SQL             string                   // 生成的SQL
	Columns         []string                 // 结果列名
	Data            []map[string]interface{} // 结果数据（可能被限制）
	RowCount        int                      // 返回的行数
	TotalRowCount   int                      // 完整行数（如果数据被限制）
	Explanation     string                   // SQL解释
	Error           string                   // 错误信息（如果有）
	Documents       []*schema.Document       // 转换为Document格式的结果（用于Chat引用）
	IntentType      string                   // 意图类型
	NeedLLMAnalysis bool                     // 是否需要LLM分析
	AnalysisFocus   []string                 // 分析重点
	DataTruncated   bool                     // 数据是否被截断
	FileURL         string                   // TODO: 大结果集文件下载URL（未来实现）
}

// DetectAndExecute 检测问题并执行NL2SQL查询
// 如果检测到是数据查询类问题，将执行NL2SQL流程并返回结果
func (t *NL2SQLTool) DetectAndExecute(ctx context.Context, question string, datasourceID string, modelID string, embeddingModelID string) (*NL2SQLResult, error) {
	g.Log().Infof(ctx, "NL2SQL Tool - Starting detection and execution for datasource: %s", datasourceID)

	result := &NL2SQLResult{
		IsNL2SQLQuery: false,
	}

	// 1. 检查数据源是否存在
	db := dao.GetDB()
	var ds dbgorm.NL2SQLDataSource
	if err := db.First(&ds, "id = ?", datasourceID).Error; err != nil {
		g.Log().Warningf(ctx, "NL2SQL Tool - Datasource not found: %s, error: %v", datasourceID, err)
		return result, nil // 数据源不存在，不是NL2SQL查询
	}

	// 2. 检查是否有已解析的表
	var parsedTableCount int64
	if err := db.Model(&dbgorm.NL2SQLTable{}).
		Where("datasource_id = ? AND parsed = ?", datasourceID, true).
		Count(&parsedTableCount).Error; err != nil {
		g.Log().Warningf(ctx, "NL2SQL Tool - Failed to check table status: %v", err)
		return result, nil
	}
	if parsedTableCount == 0 {
		g.Log().Warningf(ctx, "NL2SQL Tool - No parsed tables in datasource: %s", datasourceID)
		return result, nil // 没有已解析的表，不执行NL2SQL
	}

	g.Log().Infof(ctx, "NL2SQL Tool - Found %d parsed tables, proceeding with NL2SQL query", parsedTableCount)

	// 3. 获取模型配置（用于LLM）
	llmModelConfig, err := t.getModelConfig(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("获取LLM模型配置失败: %w", err)
	}

	// 4. 创建LLM适配器
	llmAdapter := adapter.NewLLMAdapter(llmModelConfig)

	// 5.分析用户意图
	intent, err := t.AnalyzeUserIntent(ctx, question, llmAdapter)
	if err != nil {
		g.Log().Warningf(ctx, "Failed to analyze intent: %v, using default", err)
		intent = &UserIntent{
			IntentType:  "need_analysis",
			NeedExplain: true,
			DataLimit:   10,
		}
	}

	// 记录意图到结果中
	result.IntentType = intent.IntentType
	result.NeedLLMAnalysis = intent.NeedExplain || intent.IntentType != "data_only"
	result.AnalysisFocus = intent.AnalysisFocus

	// 6. 如果只需要SQL，不执行查询
	if intent.NeedSQLOnly {
		g.Log().Infof(ctx, "User only needs SQL, generating SQL without execution")
		// TODO: 实现只生成SQL的逻辑（需要调用generator.Generate）
		result.IsNL2SQLQuery = true
		result.Error = "仅生成SQL功能尚未实现，请稍后"
		return result, nil
	}

	// 7. 创建向量搜索适配器（如果有embedding模型）
	vectorModelID := embeddingModelID
	if ds.EmbeddingModelID != "" {
		vectorModelID = ds.EmbeddingModelID
	}

	var vectorAdapter *adapter.VectorSearchAdapter
	if vectorModelID != "" {
		collectionName := fmt.Sprintf("nl2sql_%s", datasourceID)
		embeddingModelConfig, err := t.getModelConfig(ctx, vectorModelID)
		if err != nil {
			g.Log().Warningf(ctx, "NL2SQL Tool - Failed to get embedding model config: %v, will proceed without vector search", err)
		} else {
			vectorStore, err := service.GetVectorStore()
			if err != nil {
				g.Log().Warningf(ctx, "NL2SQL Tool - Failed to get vector store: %v, will proceed without vector search", err)
			} else {
				vectorAdapter = adapter.NewVectorSearchAdapter(vectorStore, collectionName, embeddingModelConfig)
			}
		}
	}

	// 8. 执行NL2SQL查询
	serviceReq := &nl2sqlservice.QueryRequest{
		DatasourceID: datasourceID,
		Question:     question,
		SessionID:    "", // Chat场景暂不需要SessionID
	}

	queryResp, err := t.nl2sqlService.QueryWithAdapters(ctx, serviceReq, llmAdapter, vectorAdapter)
	if err != nil {
		g.Log().Errorf(ctx, "NL2SQL Tool - Query failed: %v", err)
		result.IsNL2SQLQuery = true
		result.Error = err.Error()
		return result, nil // 返回结果而不是错误，让Chat继续处理
	}

	// 9. 构建返回结果
	result.IsNL2SQLQuery = true
	result.QueryLogID = queryResp.QueryLogID
	result.SQL = queryResp.SQL
	result.Explanation = queryResp.Explanation
	result.Error = queryResp.Error

	if queryResp.Result != nil {
		totalRows := queryResp.Result.RowCount
		result.TotalRowCount = totalRows
		result.Columns = queryResp.Result.Columns

		// 10.根据意图决定返回的数据量
		var dataToReturn []map[string]interface{}
		var dataForLLM []map[string]interface{}

		// 如果结果集很大（> 100行），生成文件并返回下载URL（所有模式都支持）
		if totalRows > 100 {
			g.Log().Infof(ctx, "Large result set (%d rows), generating export file", totalRows)

			// 创建文件导出器
			exporter := file_export.NewFileExporter("upload")

			// 导出为Excel格式（可以根据需要支持多种格式）
			exportReq := &file_export.ExportRequest{
				Format:      file_export.FormatExcel,
				Filename:    fmt.Sprintf("nl2sql_query_%s", result.QueryLogID),
				Columns:     result.Columns,
				Data:        queryResp.Result.Data, // 导出全部数据
				Title:       "NL2SQL查询结果",
				Description: fmt.Sprintf("SQL: %s\n说明: %s", result.SQL, result.Explanation),
			}

			exportResult, err := exporter.Export(ctx, exportReq)
			if err != nil {
				g.Log().Errorf(ctx, "Failed to export file: %v", err)
				result.FileURL = fmt.Sprintf("文件导出失败: %v", err)
			} else {
				result.FileURL = exportResult.FileURL
				// 将文件URL保存到数据库的Extra字段
				extraData := map[string]interface{}{
					"export_file_url": exportResult.FileURL,
				}
				extraJSON, _ := json.Marshal(extraData)
				dao.GetDB().Model(&dbgorm.NL2SQLQueryLog{}).Where("id = ?", result.QueryLogID).Update("extra", extraJSON)
				g.Log().Infof(ctx, "Export file generated and saved to DB: %s (size: %d bytes)", exportResult.FileURL, exportResult.Size)
			}
		}

		if intent.IntentType == "data_only" {
			// 用户只需要数据，返回更多数据但不传给LLM
			dataLimit := min(intent.DataLimit, totalRows)
			dataToReturn = limitData(queryResp.Result.Data, dataLimit)
			result.Data = dataToReturn
			result.RowCount = len(dataToReturn)
			result.DataTruncated = totalRows > dataLimit

			// data_only模式下，创建简化的document（不包含数据表格）
			result.Documents = t.convertToDocumentsWithoutData(queryResp)
		} else {
			// 需要分析或可视化，限制传给LLM的数据量
			llmDataLimit := min(10, intent.DataLimit) // 传给LLM最多10行
			dataForLLM = limitData(queryResp.Result.Data, llmDataLimit)

			// 返回用户请求的数据量
			userDataLimit := min(intent.DataLimit, totalRows)
			dataToReturn = limitData(queryResp.Result.Data, userDataLimit)

			result.Data = dataToReturn
			result.RowCount = len(dataToReturn)
			result.DataTruncated = totalRows > userDataLimit

			// 创建用于LLM的documents（只包含前10行）
			result.Documents = t.convertToDocumentsWithLimit(queryResp, dataForLLM, llmDataLimit)
		}

		g.Log().Infof(ctx, "NL2SQL Tool - Query completed: TotalRows=%d, ReturnedRows=%d, DataTruncated=%v, IntentType=%s",
			totalRows, result.RowCount, result.DataTruncated, intent.IntentType)
	}

	return result, nil
}

// getModelConfig 获取模型配置
func (t *NL2SQLTool) getModelConfig(ctx context.Context, modelID string) (*model.ModelConfig, error) {
	db := dao.GetDB()
	var modelEntity dbgorm.AIModel
	if err := db.First(&modelEntity, "model_id = ?", modelID).Error; err != nil {
		return nil, fmt.Errorf("模型不存在: %w", err)
	}

	return &model.ModelConfig{
		ModelID: modelEntity.ModelID,
		Name:    modelEntity.ModelName,
		APIKey:  modelEntity.APIKey,
		BaseURL: modelEntity.BaseURL,
	}, nil
}

// convertToDocuments 将NL2SQL查询结果转换为Document格式
func (t *NL2SQLTool) convertToDocuments(queryResp *nl2sqlservice.QueryResponse) []*schema.Document {
	var documents []*schema.Document

	// 创建一个文档展示SQL和解释
	sqlDoc := &schema.Document{
		Content: fmt.Sprintf("**生成的SQL:**\n```sql\n%s\n```\n\n**解释:** %s",
			queryResp.SQL, queryResp.Explanation),
		MetaData: map[string]interface{}{
			"type":         "nl2sql_query",
			"query_log_id": queryResp.QueryLogID,
			"sql":          queryResp.SQL,
		},
	}
	documents = append(documents, sqlDoc)

	// 如果有查询结果，创建一个文档展示数据
	if queryResp.Result != nil && queryResp.Result.RowCount > 0 {
		// 构建表格文本
		tableText := t.formatResultAsTable(queryResp.Result.Columns, queryResp.Result.Data)

		resultDoc := &schema.Document{
			Content: fmt.Sprintf("**查询结果 (共%d行):**\n%s", queryResp.Result.RowCount, tableText),
			MetaData: map[string]interface{}{
				"type":      "nl2sql_result",
				"row_count": queryResp.Result.RowCount,
				"columns":   queryResp.Result.Columns,
			},
		}
		documents = append(documents, resultDoc)
	}

	return documents
}

// formatResultAsTable 将查询结果格式化为Markdown表格
func (t *NL2SQLTool) formatResultAsTable(columns []string, data []map[string]interface{}) string {
	if len(data) == 0 {
		return "_(无数据)_"
	}

	// 限制显示的行数（避免结果过大）
	maxRows := 10
	displayData := data
	hasMore := false
	if len(data) > maxRows {
		displayData = data[:maxRows]
		hasMore = true
	}

	// 构建Markdown表格
	var tableText string

	// 表头
	tableText += "|"
	for _, col := range columns {
		tableText += fmt.Sprintf(" %s |", col)
	}
	tableText += "\n"

	// 分隔符
	tableText += "|"
	for range columns {
		tableText += " --- |"
	}
	tableText += "\n"

	// 数据行
	for _, row := range displayData {
		tableText += "|"
		for _, col := range columns {
			value := row[col]
			tableText += fmt.Sprintf(" %v |", value)
		}
		tableText += "\n"
	}

	if hasMore {
		tableText += fmt.Sprintf("\n_(显示前%d行，共%d行)_", maxRows, len(data))
	}

	return tableText
}

// AnalyzeUserIntent 分析用户意图
func (t *NL2SQLTool) AnalyzeUserIntent(ctx context.Context, question string, llmAdapter *adapter.LLMAdapter) (*UserIntent, error) {
	g.Log().Infof(ctx, "NL2SQL Tool - Analyzing user intent for question: %s", question)

	prompt := fmt.Sprintf(`分析用户问题的意图，返回JSON格式。

用户问题：%s

请判断：
1. intent_type:
   - "data_only": 用户只需要查询结果数据（如："查询所有订单"、"列出用户列表"、"导出销售数据"）
   - "need_analysis": 用户需要对结果进行分析解释（如："分析销售趋势"、"为什么销量下降"、"总结用户特征"）
   - "need_visualization": 用户可能需要可视化或图表展示（如："对比各地区销售"、"展示增长曲线"）

2. need_sql_only: 是否只需要SQL语句，不执行（如："写一个SQL查询..."、"帮我生成查询语句"）

3. need_explain: 是否需要解释结果的含义和业务洞察

4. data_limit: 建议返回的数据行数
   - data_only时: 可返回较多数据（如100-1000行）
   - need_analysis时: 建议10-20行用于分析
   - need_visualization时: 建议20-50行用于图表

5. analysis_focus: 如果需要分析，用户关注什么（如：["趋势", "异常值", "对比", "原因分析"]）

只返回JSON，不要其他解释，格式如：
{
  "intent_type": "data_only",
  "need_sql_only": false,
  "need_explain": false,
  "data_limit": 100,
  "analysis_focus": []
}`, question)

	response, err := llmAdapter.Call(ctx, prompt)
	if err != nil {
		g.Log().Warningf(ctx, "Failed to analyze intent: %v, using default", err)
		// 降级：默认返回需要分析的意图
		return &UserIntent{
			IntentType:  "need_analysis",
			NeedExplain: true,
			DataLimit:   10,
		}, nil
	}

	g.Log().Debugf(ctx, "LLM intent response: %s", response)

	// 解析JSON（尝试提取JSON部分）
	var intent UserIntent
	// 尝试找到JSON部分
	jsonStart := -1
	jsonEnd := -1
	for i, ch := range response {
		if ch == '{' && jsonStart == -1 {
			jsonStart = i
		}
		if ch == '}' {
			jsonEnd = i + 1
		}
	}

	if jsonStart != -1 && jsonEnd != -1 {
		jsonStr := response[jsonStart:jsonEnd]
		if err := json.Unmarshal([]byte(jsonStr), &intent); err != nil {
			g.Log().Warningf(ctx, "Failed to parse intent JSON: %v, using default", err)
			return &UserIntent{
				IntentType:  "need_analysis",
				NeedExplain: true,
				DataLimit:   10,
			}, nil
		}
	} else {
		g.Log().Warningf(ctx, "No valid JSON found in response, using default")
		return &UserIntent{
			IntentType:  "need_analysis",
			NeedExplain: true,
			DataLimit:   10,
		}, nil
	}

	// 设置合理的默认值
	if intent.DataLimit == 0 {
		if intent.IntentType == "data_only" {
			intent.DataLimit = 100
		} else {
			intent.DataLimit = 10
		}
	}

	// 限制最大值
	if intent.DataLimit > 1000 {
		intent.DataLimit = 1000
	}

	g.Log().Infof(ctx, "Intent analyzed: type=%s, need_explain=%v, data_limit=%d, focus=%v",
		intent.IntentType, intent.NeedExplain, intent.DataLimit, intent.AnalysisFocus)

	return &intent, nil
}

// limitData 限制返回的数据行数
func limitData(data []map[string]interface{}, limit int) []map[string]interface{} {
	if len(data) <= limit {
		return data
	}
	return data[:limit]
}

// min 返回两个整数的最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// convertToDocumentsWithoutData 转换为Document格式（不包含数据表格，用于data_only模式）
func (t *NL2SQLTool) convertToDocumentsWithoutData(queryResp *nl2sqlservice.QueryResponse) []*schema.Document {
	var documents []*schema.Document

	// 只创建SQL文档，不包含数据表格
	sqlDoc := &schema.Document{
		Content: fmt.Sprintf("**生成的SQL:**\n```sql\n%s\n```\n\n**解释:** %s\n\n**说明:** 查询结果数据已返回，共 %d 行。",
			queryResp.SQL, queryResp.Explanation, queryResp.Result.RowCount),
		MetaData: map[string]interface{}{
			"type":         "nl2sql_query",
			"query_log_id": queryResp.QueryLogID,
			"sql":          queryResp.SQL,
			"mode":         "data_only",
		},
	}
	documents = append(documents, sqlDoc)

	return documents
}

// convertToDocumentsWithLimit 转换为Document格式（限制数据量，用于need_analysis模式）
func (t *NL2SQLTool) convertToDocumentsWithLimit(queryResp *nl2sqlservice.QueryResponse, limitedData []map[string]interface{}, dataLimit int) []*schema.Document {
	var documents []*schema.Document

	// 创建SQL文档
	sqlDoc := &schema.Document{
		Content: fmt.Sprintf("**生成的SQL:**\n```sql\n%s\n```\n\n**解释:** %s",
			queryResp.SQL, queryResp.Explanation),
		MetaData: map[string]interface{}{
			"type":         "nl2sql_query",
			"query_log_id": queryResp.QueryLogID,
			"sql":          queryResp.SQL,
		},
	}
	documents = append(documents, sqlDoc)

	// 创建数据文档（使用限制后的数据）
	if len(limitedData) > 0 {
		tableText := t.formatResultAsTableForLLM(queryResp.Result.Columns, limitedData)

		resultDoc := &schema.Document{
			Content: fmt.Sprintf("**查询结果 (显示前%d行，共%d行):**\n%s\n\n**分析提示:** 请基于以上数据进行分析。",
				len(limitedData), queryResp.Result.RowCount, tableText),
			MetaData: map[string]interface{}{
				"type":       "nl2sql_result",
				"row_count":  queryResp.Result.RowCount,
				"shown_rows": len(limitedData),
				"columns":    queryResp.Result.Columns,
			},
		}
		documents = append(documents, resultDoc)
	}

	return documents
}

// formatResultAsTableForLLM 格式化结果为表格（专门用于LLM，不做额外限制）
func (t *NL2SQLTool) formatResultAsTableForLLM(columns []string, data []map[string]interface{}) string {
	if len(data) == 0 {
		return "_(无数据)_"
	}

	// 构建Markdown表格
	var tableText string

	// 表头
	tableText += "|"
	for _, col := range columns {
		tableText += fmt.Sprintf(" %s |", col)
	}
	tableText += "\n"

	// 分隔符
	tableText += "|"
	for range columns {
		tableText += " --- |"
	}
	tableText += "\n"

	// 数据行
	for _, row := range data {
		tableText += "|"
		for _, col := range columns {
			value := row[col]
			tableText += fmt.Sprintf(" %v |", value)
		}
		tableText += "\n"
	}

	return tableText
}
