package generator

import (
	"context"
	"fmt"
	"strings"

	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"gorm.io/gorm"
)

// SQLGenerator SQL生成器
type SQLGenerator struct {
	db *gorm.DB
}

// NewSQLGenerator 创建SQL生成器
func NewSQLGenerator(db *gorm.DB) *SQLGenerator {
	return &SQLGenerator{
		db: db,
	}
}

// GenerateRequest SQL生成请求
type GenerateRequest struct {
	Question        string                 `json:"question"`
	SchemaContext   *SchemaContext         `json:"schema_context"`
	ConversationCtx map[string]interface{} `json:"conversation_ctx,omitempty"`
}

// SchemaContext Schema上下文
type SchemaContext struct {
	Tables    []TableContext    `json:"tables"`
	Metrics   []MetricContext   `json:"metrics"`
	Relations []RelationContext `json:"relations"`
}

// TableContext 表上下文
type TableContext struct {
	Name        string          `json:"table_name"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Columns     []ColumnContext `json:"columns"`
}

// ColumnContext 列上下文
type ColumnContext struct {
	ColumnName  string `json:"column_name"`
	DisplayName string `json:"display_name"`
	DataType    string `json:"data_type"`
	Description string `json:"description"`
}

// MetricContext 指标上下文
type MetricContext struct {
	MetricID    string `json:"metric_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Formula     string `json:"formula"`
}

// RelationContext 关系上下文
type RelationContext struct {
	FromTable string `json:"from_table"`
	FromCol   string `json:"from_col"`
	ToTable   string `json:"to_table"`
	ToCol     string `json:"to_col"`
}

// GenerateResponse SQL生成响应
type GenerateResponse struct {
	SQL          string                 `json:"sql"`
	Reasoning    string                 `json:"reasoning"`
	Confidence   float64                `json:"confidence"`
	Alternatives []string               `json:"alternatives,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Generate 生成SQL
func (g *SQLGenerator) Generate(ctx context.Context, req *GenerateRequest, llmFunc func(prompt string) (string, error)) (*GenerateResponse, error) {
	// 1. 构建Prompt
	prompt := g.buildPrompt(req)

	// 2. 调用LLM
	llmResponse, err := llmFunc(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM调用失败: %w", err)
	}

	// 3. 解析LLM响应
	response := g.parseLLMResponse(llmResponse)

	return response, nil
}

// buildPrompt 构建LLM Prompt
func (g *SQLGenerator) buildPrompt(req *GenerateRequest) string {
	var sb strings.Builder

	// 系统角色
	sb.WriteString("你是一个专业的SQL生成助手。根据用户问题和数据库Schema，生成准确的SQL查询。\n\n")

	// Schema信息
	sb.WriteString("## 数据库Schema\n\n")

	// 表信息
	for _, table := range req.SchemaContext.Tables {
		sb.WriteString(fmt.Sprintf("###表: %s (%s)\n", table.Name, table.DisplayName))
		if table.Description != "" {
			sb.WriteString(fmt.Sprintf("说明: %s\n", table.Description))
		}
		sb.WriteString("字段:\n")
		for _, col := range table.Columns {
			sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", col.ColumnName, col.DataType, col.Description))
		}
		sb.WriteString("\n")
	}

	// 指标信息
	if len(req.SchemaContext.Metrics) > 0 {
		sb.WriteString("### 预定义指标\n")
		for _, metric := range req.SchemaContext.Metrics {
			sb.WriteString(fmt.Sprintf("- %s: %s (公式: %s)\n", metric.Name, metric.Description, metric.Formula))
		}
		sb.WriteString("\n")
	}

	// 关系信息
	if len(req.SchemaContext.Relations) > 0 {
		sb.WriteString("### 表关系\n")
		for _, rel := range req.SchemaContext.Relations {
			sb.WriteString(fmt.Sprintf("- %s.%s -> %s.%s\n", rel.FromTable, rel.FromCol, rel.ToTable, rel.ToCol))
		}
		sb.WriteString("\n")
	}

	// 规则约束
	sb.WriteString("## 规则\n")
	sb.WriteString("1. 只生成SELECT查询，禁止INSERT/UPDATE/DELETE\n")
	sb.WriteString("2. 始终添加LIMIT子句限制结果数量（默认1000）\n")
	sb.WriteString("3. 优先使用预定义指标\n")
	sb.WriteString("4. 时间过滤优先使用相对时间（如 CURRENT_DATE - INTERVAL '7 days'）\n")
	sb.WriteString("5. 避免SELECT *，明确列出所需字段\n")
	sb.WriteString("6. 确保JOIN条件正确\n\n")

	// 用户问题
	sb.WriteString("## 用户问题\n")
	sb.WriteString(req.Question)
	sb.WriteString("\n\n")

	// 输出格式
	sb.WriteString("## 输出格式\n")
	sb.WriteString("请按以下JSON格式返回：\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"sql\": \"生成的SQL语句\",\n")
	sb.WriteString("  \"reasoning\": \"生成思路说明\",\n")
	sb.WriteString("  \"confidence\": 0.95\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")

	return sb.String()
}

// parseLLMResponse 解析LLM响应
func (g *SQLGenerator) parseLLMResponse(llmResponse string) *GenerateResponse {
	// 简化实现：尝试提取JSON
	// 完整实现需要更健壮的JSON解析

	response := &GenerateResponse{
		SQL:        "",
		Reasoning:  "",
		Confidence: 0.8,
		Metadata:   make(map[string]interface{}),
	}

	// 尝试从```json代码块中提取
	if strings.Contains(llmResponse, "```json") {
		start := strings.Index(llmResponse, "```json") + 7
		end := strings.Index(llmResponse[start:], "```")
		if end > 0 {
			jsonStr := llmResponse[start : start+end]
			// TODO: 解析JSON
			_ = jsonStr
		}
	}

	// 简化：直接返回原始响应
	response.SQL = llmResponse
	response.Reasoning = "LLM生成的SQL"

	return response
}

// RecallSchema 召回相关Schema（向量检索）
func (g *SQLGenerator) RecallSchema(ctx context.Context, schemaID, question string) (*SchemaContext, error) {
	// 1. 使用向量检索召回相关表
	// TODO: 集成向量检索

	// 2. 简化实现：返回所有表（MVP版本）
	var tables []dbgorm.NL2SQLTable
	if err := g.db.Where("schema_id = ?", schemaID).Limit(5).Find(&tables).Error; err != nil {
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
			Name:        table.Name,
			DisplayName: table.DisplayName,
			Description: table.Description,
			Columns:     make([]ColumnContext, 0),
		}

		// 获取列信息
		var columns []dbgorm.NL2SQLColumn
		if err := g.db.Where("table_id = ?", table.ID).Find(&columns).Error; err != nil {
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
	if err := g.db.Where("schema_id = ?", schemaID).Find(&relations).Error; err == nil {
		for _, rel := range relations {
			// 获取表名
			var fromTable, toTable dbgorm.NL2SQLTable
			g.db.First(&fromTable, "id = ?", rel.FromTableID)
			g.db.First(&toTable, "id = ?", rel.ToTableID)

			schemaCtx.Relations = append(schemaCtx.Relations, RelationContext{
				FromTable: fromTable.Name,
				FromCol:   rel.FromColumn,
				ToTable:   toTable.Name,
				ToCol:     rel.ToColumn,
			})
		}
	}

	return schemaCtx, nil
}
