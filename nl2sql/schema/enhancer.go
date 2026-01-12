package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/core/model"
	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	pkgSchema "github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// SchemaEnhancer LLM增强Schema描述
type SchemaEnhancer struct {
	db           *gorm.DB
	modelService *model.ModelService
	modelConfig  *model.ModelConfig
}

// NewSchemaEnhancer 创建Schema增强器
func NewSchemaEnhancer(db *gorm.DB, modelID string) (*SchemaEnhancer, error) {
	// 获取模型配置
	modelConfig := model.Registry.Get(modelID)
	if modelConfig == nil {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}

	// 创建模型服务
	modelService := model.NewModelService(
		modelConfig.APIKey,
		modelConfig.BaseURL,
		nil, // 使用默认formatter
	)

	return &SchemaEnhancer{
		db:           db,
		modelService: modelService,
		modelConfig:  modelConfig,
	}, nil
}

// EnhanceSchemaRequest 增强Schema请求
type EnhanceSchemaRequest struct {
	DatasourceID string
}

// EnhanceSchema 增强Schema的所有描述
func (e *SchemaEnhancer) EnhanceSchema(ctx context.Context, req *EnhanceSchemaRequest) error {
	g.Log().Infof(ctx, "开始LLM增强Schema - DatasourceID: %s", req.DatasourceID)

	// 1. 获取所有表
	var tables []dbgorm.NL2SQLTable
	if err := e.db.Where("datasource_id = ?", req.DatasourceID).Find(&tables).Error; err != nil {
		return fmt.Errorf("获取表列表失败: %w", err)
	}

	if len(tables) == 0 {
		g.Log().Warningf(ctx, "数据源 %s 没有表，跳过增强", req.DatasourceID)
		return nil
	}

	g.Log().Infof(ctx, "找到 %d 个表需要增强", len(tables))

	// 2. 批量增强表描述
	for i := range tables {
		table := &tables[i]
		if err := e.enhanceTable(ctx, table); err != nil {
			g.Log().Warningf(ctx, "增强表 %s 失败: %v", table.DisplayName, err)
			// 继续处理下一个表
			continue
		}
		g.Log().Infof(ctx, "成功增强表: %s", table.DisplayName)
	}

	// 3. 生成表关系（Relations）
	g.Log().Infof(ctx, "开始生成表关系 - DatasourceID: %s", req.DatasourceID)
	if err := e.generateRelations(ctx, req.DatasourceID, tables); err != nil {
		g.Log().Warningf(ctx, "生成表关系失败: %v", err)
		// Relations生成失败不影响整体流程
	} else {
		g.Log().Infof(ctx, "表关系生成成功")
	}

	// 4. 生成业务指标（Metrics）
	g.Log().Infof(ctx, "开始生成业务指标 - DatasourceID: %s", req.DatasourceID)
	if err := e.generateMetrics(ctx, req.DatasourceID, tables); err != nil {
		g.Log().Warningf(ctx, "生成业务指标失败: %v", err)
		// Metrics生成失败不影响整体流程
	} else {
		g.Log().Infof(ctx, "业务指标生成成功")
	}

	g.Log().Infof(ctx, "Schema增强完成 - DatasourceID: %s", req.DatasourceID)
	return nil
}

// enhanceTable 增强单个表的描述
func (e *SchemaEnhancer) enhanceTable(ctx context.Context, table *dbgorm.NL2SQLTable) error {
	// 1. 获取表的所有列
	var columns []dbgorm.NL2SQLColumn
	if err := e.db.Where("table_id = ?", table.ID).Find(&columns).Error; err != nil {
		return fmt.Errorf("获取列信息失败: %w", err)
	}

	// 2. 构建列信息列表
	columnNames := make([]string, len(columns))
	columnDetails := make([]string, len(columns))
	for i, col := range columns {
		columnNames[i] = col.ColumnName
		columnDetails[i] = fmt.Sprintf("%s (%s)", col.ColumnName, col.DataType)
	}

	// 3. 调用LLM生成表描述
	tableDesc, err := e.generateTableDescription(ctx, table.DisplayName, columnDetails)
	if err != nil {
		return fmt.Errorf("生成表描述失败: %w", err)
	}

	// 4. 更新表描述
	if tableDesc != "" {
		if err := e.db.Model(table).Update("description", tableDesc).Error; err != nil {
			return fmt.Errorf("更新表描述失败: %w", err)
		}
	}

	// 5. 增强每个列的描述
	for i := range columns {
		col := &columns[i]
		if err := e.enhanceColumn(ctx, table.DisplayName, col); err != nil {
			g.Log().Warningf(ctx, "增强列 %s.%s 失败: %v", table.DisplayName, col.ColumnName, err)
			continue
		}
	}

	return nil
}

// generateTableDescription 使用LLM生成表描述
func (e *SchemaEnhancer) generateTableDescription(ctx context.Context, tableName string, columnDetails []string) (string, error) {
	prompt := fmt.Sprintf(`你是一个数据库专家，请根据以下信息生成表的业务描述：

表名: %s
字段列表:
%s

请用1-2句话描述这个表的业务含义和用途。
只返回描述文本，不要其他内容。`, tableName, strings.Join(columnDetails, "\n"))

	// 调用LLM
	params := model.ChatCompletionParams{
		ModelName:   e.modelConfig.Name,
		Messages:    buildMessages(prompt),
		Temperature: 0.3, // 低温度保证结果稳定
	}

	response, err := e.modelService.ChatCompletion(ctx, params)
	if err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("LLM返回空响应")
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

// ColumnEnhanceResult 列增强结果
type ColumnEnhanceResult struct {
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	SemanticType string `json:"semantic_type"`
	Unit         string `json:"unit,omitempty"`
}

// enhanceColumn 增强单个列的描述
func (e *SchemaEnhancer) enhanceColumn(ctx context.Context, tableName string, column *dbgorm.NL2SQLColumn) error {
	prompt := fmt.Sprintf(`你是一个数据库专家，请分析以下字段：

表名: %s
字段名: %s
数据类型: %s

请返回JSON格式：
{
  "display_name": "中文显示名称",
  "description": "详细描述（1句话）",
  "semantic_type": "语义类型(id/currency/time/category/text/number之一)",
  "unit": "单位（如果是数值或货币，可选）"
}

只返回JSON，不要其他内容。`, tableName, column.ColumnName, column.DataType)

	// 调用LLM
	params := model.ChatCompletionParams{
		ModelName:   e.modelConfig.Name,
		Messages:    buildMessages(prompt),
		Temperature: 0.3,
	}

	response, err := e.modelService.ChatCompletion(ctx, params)
	if err != nil {
		return err
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("LLM返回空响应")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)

	// 提取JSON（如果LLM返回了额外的文本）
	content = extractJSON(content)

	// 解析JSON
	var result ColumnEnhanceResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		g.Log().Warningf(ctx, "解析LLM响应失败: %v, 内容: %s", err, content)
		return fmt.Errorf("解析LLM响应失败: %w", err)
	}

	// 更新列信息
	updates := map[string]interface{}{}
	if result.DisplayName != "" {
		updates["display_name"] = result.DisplayName
	}
	if result.Description != "" {
		updates["description"] = result.Description
	}
	if result.SemanticType != "" {
		updates["semantic_type"] = result.SemanticType
	}
	if result.Unit != "" {
		updates["unit"] = &result.Unit
	}

	if len(updates) > 0 {
		if err := e.db.Model(column).Updates(updates).Error; err != nil {
			return fmt.Errorf("更新列信息失败: %w", err)
		}
	}

	return nil
}

// buildMessages 构建消息数组
func buildMessages(prompt string) []*pkgSchema.Message {
	return []*pkgSchema.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}
}

// extractJSON 从文本中提取JSON（处理LLM可能返回的markdown代码块）
func extractJSON(text string) string {
	text = strings.TrimSpace(text)

	// 移除markdown代码块标记
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
	}

	return strings.TrimSpace(text)
}

// RelationSuggestion LLM建议的关系
type RelationSuggestion struct {
	FromTable    string `json:"from_table"`
	FromColumn   string `json:"from_column"`
	ToTable      string `json:"to_table"`
	ToColumn     string `json:"to_column"`
	RelationType string `json:"relation_type"` // many_to_one, one_to_many, one_to_one
	Description  string `json:"description"`
}

// generateRelations 使用LLM分析并生成表之间的关系
func (e *SchemaEnhancer) generateRelations(ctx context.Context, datasourceID string, tables []dbgorm.NL2SQLTable) error {
	if len(tables) < 2 {
		g.Log().Infof(ctx, "表数量少于2个，跳过关系生成")
		return nil
	}

	// 先检查是否已存在关系
	var existingRelations []dbgorm.NL2SQLRelation
	if err := e.db.Where("datasource_id = ?", datasourceID).Find(&existingRelations).Error; err != nil {
		return fmt.Errorf("检查已有关系失败: %w", err)
	}

	if len(existingRelations) > 0 {
		g.Log().Infof(ctx, "已存在 %d 个关系，跳过生成", len(existingRelations))
		return nil
	}

	// 1. 为每个表获取列信息
	tableSchemas := make([]string, 0, len(tables))
	tableMap := make(map[string]*dbgorm.NL2SQLTable)

	for i := range tables {
		table := &tables[i]
		tableMap[table.DisplayName] = table

		var columns []dbgorm.NL2SQLColumn
		if err := e.db.Where("table_id = ?", table.ID).Find(&columns).Error; err != nil {
			continue
		}

		columnList := make([]string, len(columns))
		for j, col := range columns {
			columnList[j] = fmt.Sprintf("  - %s (%s)", col.ColumnName, col.DataType)
		}

		tableSchema := fmt.Sprintf("表: %s\n%s", table.DisplayName, strings.Join(columnList, "\n"))
		tableSchemas = append(tableSchemas, tableSchema)
	}

	// 2. 构建prompt
	prompt := fmt.Sprintf(`你是一个数据库专家，请分析以下数据库表的结构，识别表之间可能存在的外键关系：

%s

请识别表之间的关系（如订单表的user_id关联用户表的id），并返回JSON数组格式：
[
  {
    "from_table": "源表名",
    "from_column": "源表列名",
    "to_table": "目标表名",
    "to_column": "目标表列名",
    "relation_type": "关系类型(many_to_one/one_to_many/one_to_one)",
    "description": "关系描述"
  }
]

注意：
1. 只识别明确的外键关系（如id、user_id等）
2. 如果没有明确的关系，返回空数组 []
3. 关系类型：many_to_one表示多对一（如多个订单属于一个用户），one_to_many表示一对多，one_to_one表示一对一
4. 只返回JSON数组，不要其他内容`, strings.Join(tableSchemas, "\n\n"))

	// 3. 调用LLM
	params := model.ChatCompletionParams{
		ModelName:   e.modelConfig.Name,
		Messages:    buildMessages(prompt),
		Temperature: 0.2, // 更低的温度确保稳定输出
	} // 可以添加ResponseFormat测试

	response, err := e.modelService.ChatCompletion(ctx, params)
	if err != nil {
		return fmt.Errorf("调用LLM失败: %w", err)
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("LLM返回空响应")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	content = extractJSON(content)

	// 4. 解析JSON
	var suggestions []RelationSuggestion
	if err := json.Unmarshal([]byte(content), &suggestions); err != nil {
		g.Log().Warningf(ctx, "解析LLM关系建议失败: %v, 内容: %s", err, content)
		return fmt.Errorf("解析LLM响应失败: %w", err)
	}

	if len(suggestions) == 0 {
		g.Log().Infof(ctx, "LLM未识别到表关系")
		return nil
	}

	// 5. 保存关系到数据库
	createdCount := 0
	for _, suggestion := range suggestions {
		// 验证表名是否存在
		fromTable, fromExists := tableMap[suggestion.FromTable]
		toTable, toExists := tableMap[suggestion.ToTable]

		if !fromExists || !toExists {
			g.Log().Warningf(ctx, "跳过无效关系: %s.%s -> %s.%s (表不存在)",
				suggestion.FromTable, suggestion.FromColumn,
				suggestion.ToTable, suggestion.ToColumn)
			continue
		}

		// 验证列是否存在
		var fromColCount, toColCount int64
		e.db.Model(&dbgorm.NL2SQLColumn{}).
			Where("table_id = ? AND column_name = ?", fromTable.ID, suggestion.FromColumn).
			Count(&fromColCount)
		e.db.Model(&dbgorm.NL2SQLColumn{}).
			Where("table_id = ? AND column_name = ?", toTable.ID, suggestion.ToColumn).
			Count(&toColCount)

		if fromColCount == 0 || toColCount == 0 {
			g.Log().Warningf(ctx, "跳过无效关系: %s.%s -> %s.%s (列不存在)",
				suggestion.FromTable, suggestion.FromColumn,
				suggestion.ToTable, suggestion.ToColumn)
			continue
		}

		// 创建关系记录
		relation := &dbgorm.NL2SQLRelation{
			DatasourceID: datasourceID,
			RelationID:   fmt.Sprintf("rel_%s_%s_to_%s_%s", suggestion.FromTable, suggestion.FromColumn, suggestion.ToTable, suggestion.ToColumn),
			FromTableID:  fromTable.ID,
			FromColumn:   suggestion.FromColumn,
			ToTableID:    toTable.ID,
			ToColumn:     suggestion.ToColumn,
			RelationType: suggestion.RelationType,
			JoinType:     "INNER", // 默认INNER JOIN
			Description:  suggestion.Description,
		}

		if err := e.db.Create(relation).Error; err != nil {
			g.Log().Warningf(ctx, "保存关系失败: %v", err)
			continue
		}

		createdCount++
		g.Log().Infof(ctx, "创建关系: %s.%s -> %s.%s (%s)",
			suggestion.FromTable, suggestion.FromColumn,
			suggestion.ToTable, suggestion.ToColumn,
			suggestion.RelationType)
	}

	g.Log().Infof(ctx, "成功生成 %d 个表关系", createdCount)
	return nil
}

// MetricSuggestion LLM建议的指标
type MetricSuggestion struct {
	MetricCode     string   `json:"metric_id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Formula        string   `json:"formula"`
	DefaultFilters []string `json:"default_filters,omitempty"`
	TimeColumn     string   `json:"time_column,omitempty"`
}

// generateMetrics 使用LLM分析并生成业务指标
func (e *SchemaEnhancer) generateMetrics(ctx context.Context, datasourceID string, tables []dbgorm.NL2SQLTable) error {
	// 先检查是否已存在指标
	var existingMetrics []dbgorm.NL2SQLMetric
	if err := e.db.Where("datasource_id = ?", datasourceID).Find(&existingMetrics).Error; err != nil {
		return fmt.Errorf("检查已有指标失败: %w", err)
	}

	if len(existingMetrics) > 0 {
		g.Log().Infof(ctx, "已存在 %d 个指标，跳过生成", len(existingMetrics))
		return nil
	}

	// 1. 收集所有表和列信息
	tableSchemas := make([]string, 0, len(tables))

	for i := range tables {
		table := &tables[i]

		var columns []dbgorm.NL2SQLColumn
		if err := e.db.Where("table_id = ?", table.ID).Find(&columns).Error; err != nil {
			continue
		}

		columnList := make([]string, len(columns))
		for j, col := range columns {
			desc := col.ColumnName
			if col.Description != "" {
				desc += fmt.Sprintf(" (%s, %s)", col.DataType, col.Description)
			} else {
				desc += fmt.Sprintf(" (%s)", col.DataType)
			}
			columnList[j] = fmt.Sprintf("  - %s", desc)
		}

		tableDesc := table.DisplayName
		if table.Description != "" {
			tableDesc += fmt.Sprintf(" - %s", table.Description)
		}

		tableSchema := fmt.Sprintf("表: %s\n%s", tableDesc, strings.Join(columnList, "\n"))
		tableSchemas = append(tableSchemas, tableSchema)
	}

	// 2. 构建prompt
	prompt := fmt.Sprintf(`你是一个数据分析专家，请根据以下数据库表结构，建议3-5个常用的业务指标：

%s

请返回JSON数组格式的指标定义：
[
  {
    "metric_id": "指标唯一标识（英文，如metric_total_revenue）",
    "name": "指标名称（中文）",
    "description": "指标描述",
    "formula": "计算公式（SQL聚合表达式，如SUM(orders.amount)）",
    "default_filters": ["默认过滤条件数组（可选，如orders.status = 'paid'）"],
    "time_column": "时间列（可选，如orders.created_at）"
  }
]

注意：
1. 指标应该是有业务意义的聚合计算（如总金额、订单数、平均客单价等）
2. 公式要使用标准SQL聚合函数（SUM、COUNT、AVG等）
3. 如果表结构不适合生成指标，返回空数组 []
4. 只返回JSON数组，不要其他内容`, strings.Join(tableSchemas, "\n\n"))

	// 3. 调用LLM
	params := model.ChatCompletionParams{
		ModelName:   e.modelConfig.Name,
		Messages:    buildMessages(prompt),
		Temperature: 0.3,
	} // 可以添加ResponseFormat测试

	response, err := e.modelService.ChatCompletion(ctx, params)
	if err != nil {
		return fmt.Errorf("调用LLM失败: %w", err)
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("LLM返回空响应")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	content = extractJSON(content)

	// 4. 解析JSON
	var suggestions []MetricSuggestion
	if err := json.Unmarshal([]byte(content), &suggestions); err != nil {
		g.Log().Warningf(ctx, "解析LLM指标建议失败: %v, 内容: %s", err, content)
		return fmt.Errorf("解析LLM响应失败: %w", err)
	}

	if len(suggestions) == 0 {
		g.Log().Infof(ctx, "LLM未建议任何指标")
		return nil
	}

	// 5. 保存指标到数据库
	createdCount := 0
	for _, suggestion := range suggestions {
		// 构建默认过滤条件JSON
		var defaultFiltersJSON []byte
		if len(suggestion.DefaultFilters) > 0 {
			filtersMap := make(map[string]string)
			for _, filter := range suggestion.DefaultFilters {
				// 简单解析 "column = 'value'" 格式
				parts := strings.SplitN(filter, "=", 2)
				if len(parts) == 2 {
					filtersMap[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), "'\"")
				}
			}
			defaultFiltersJSON, _ = json.Marshal(filtersMap)
		}

		metric := &dbgorm.NL2SQLMetric{
			DatasourceID:   datasourceID,
			MetricCode:     suggestion.MetricCode,
			Name:           suggestion.Name,
			Description:    suggestion.Description,
			Formula:        suggestion.Formula,
			DefaultFilters: defaultFiltersJSON,
			TimeColumn:     suggestion.TimeColumn,
		}

		if err := e.db.Create(metric).Error; err != nil {
			g.Log().Warningf(ctx, "保存指标失败: %v", err)
			continue
		}

		createdCount++
		g.Log().Infof(ctx, "创建指标: %s (%s)", suggestion.Name, suggestion.MetricCode)
	}

	g.Log().Infof(ctx, "成功生成 %d 个业务指标", createdCount)
	return nil
}
