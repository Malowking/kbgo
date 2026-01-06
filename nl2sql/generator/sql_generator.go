package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
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
	response := &GenerateResponse{
		SQL:        "",
		Reasoning:  "",
		Confidence: 0.8,
		Metadata:   make(map[string]interface{}),
	}

	// 尝试从```json代码块中提取
	jsonStr := ""
	if strings.Contains(llmResponse, "```json") {
		start := strings.Index(llmResponse, "```json") + 7
		end := strings.Index(llmResponse[start:], "```")
		if end > 0 {
			jsonStr = strings.TrimSpace(llmResponse[start : start+end])
		}
	} else if strings.Contains(llmResponse, "```") {
		// 尝试提取普通代码块
		start := strings.Index(llmResponse, "```") + 3
		end := strings.Index(llmResponse[start:], "```")
		if end > 0 {
			jsonStr = strings.TrimSpace(llmResponse[start : start+end])
		}
	} else {
		// 尝试直接解析整个响应
		jsonStr = strings.TrimSpace(llmResponse)
	}

	// 解析JSON
	if jsonStr != "" {
		var parsed struct {
			SQL        string  `json:"sql"`
			Reasoning  string  `json:"reasoning"`
			Confidence float64 `json:"confidence"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			// 成功解析JSON
			response.SQL = parsed.SQL
			response.Reasoning = parsed.Reasoning
			if parsed.Confidence > 0 {
				response.Confidence = parsed.Confidence
			}
			return response
		}
	}

	// 如果JSON解析失败，尝试智能提取SQL
	// 查找SQL关键字
	lowerResponse := strings.ToLower(llmResponse)
	if strings.Contains(lowerResponse, "select") {
		// 尝试提取SQL语句
		lines := strings.Split(llmResponse, "\n")
		var sqlLines []string
		inSQL := false

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			lowerLine := strings.ToLower(trimmed)

			// 检测SQL开始
			if strings.HasPrefix(lowerLine, "select") {
				inSQL = true
			}

			if inSQL {
				// 跳过代码块标记
				if strings.HasPrefix(trimmed, "```") {
					continue
				}
				sqlLines = append(sqlLines, trimmed)

				// 检测SQL结束（分号或空行）
				if strings.HasSuffix(trimmed, ";") {
					break
				}
			}
		}

		if len(sqlLines) > 0 {
			response.SQL = strings.Join(sqlLines, " ")
			response.Reasoning = "从LLM响应中提取的SQL"
			return response
		}
	}

	// 最后的fallback：返回原始响应
	response.SQL = llmResponse
	response.Reasoning = "LLM原始响应（未能解析为标准格式）"

	return response
}

// RecallSchema 召回相关Schema（向量检索）
func (gen *SQLGenerator) RecallSchema(ctx context.Context, schemaID, question string) (*SchemaContext, error) {
	g.Log().Infof(ctx, "RecallSchema开始 - SchemaID: %s, Question: %s", schemaID, question)

	// 1. 尝试使用向量检索
	schemaCtx, err := gen.recallSchemaWithVector(ctx, schemaID, question)
	if err != nil {
		g.Log().Warningf(ctx, "向量检索失败，降级到简单查询: %v", err)
		// 降级到简单实现
		return gen.recallSchemaFallback(ctx, schemaID)
	}

	if len(schemaCtx.Tables) > 0 {
		g.Log().Infof(ctx, "向量检索成功，召回 %d 个表", len(schemaCtx.Tables))
		return schemaCtx, nil
	}

	// 2. 如果向量检索没有结果，降级到简单实现
	g.Log().Warning(ctx, "向量检索无结果，降级到简单查询")
	return gen.recallSchemaFallback(ctx, schemaID)
}
