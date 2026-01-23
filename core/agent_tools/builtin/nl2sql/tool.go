package nl2sql

import (
	"fmt"

	"github.com/Malowking/kbgo/core/agent_tools/framework"
	"github.com/Malowking/kbgo/core/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// NL2SQLToolAdapter NL2SQL工具适配器
type NL2SQLToolAdapter struct {
	tool *NL2SQLTool
}

// NewNL2SQLToolAdapter 创建NL2SQL工具适配器
func NewNL2SQLToolAdapter() *NL2SQLToolAdapter {
	return &NL2SQLToolAdapter{
		tool: NewNL2SQLTool(),
	}
}

// Execute 执行NL2SQL查询（框架适配）
func (t *NL2SQLToolAdapter) Execute(
	ctx framework.ToolContext,
	config *NL2SQLConfig,
	params map[string]interface{},
) (*schema.ToolResult, error) {
	// 提取参数
	question, _ := params["query"].(string)
	datasourceID, _ := params["datasource_id"].(string)
	modelID, _ := params["model_id"].(string)

	// 如果配置中有值，优先使用配置
	if config != nil {
		if config.Question != "" {
			question = config.Question
		}
		if config.DatasourceID != "" {
			datasourceID = config.DatasourceID
		}
		if config.ModelID != "" {
			modelID = config.ModelID
		}
	}

	g.Log().Infof(ctx.Context, "[NL2SQLTool] Starting query: %s, datasource: %s", question, datasourceID)

	// 执行NL2SQL
	result, err := t.tool.DetectAndExecute(ctx.Context, question, datasourceID, modelID)
	if err != nil {
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("NL2SQL执行失败: %v", err), ""),
		), nil
	}

	// 如果不是NL2SQL查询
	if !result.IsNL2SQLQuery {
		return schema.NewToolResultFromError(
			schema.NewValidationError("该问题不适合使用NL2SQL查询", ""),
		), nil
	}

	// 如果有错误
	if result.Error != "" {
		return schema.NewToolResultFromError(
			schema.NewExecutionError(result.Error, ""),
		), nil
	}

	// Data字段存储实际的查询结果数据
	toolResult := schema.NewToolResult(result.Data)

	// Metadata字段存储执行相关的元信息
	toolResult.WithMetadata("sql", result.SQL)
	toolResult.WithMetadata("explanation", result.Explanation)
	toolResult.WithMetadata("columns", result.Columns)
	toolResult.WithMetadata("row_count", result.RowCount)
	toolResult.WithMetadata("total_row_count", result.TotalRowCount)
	toolResult.WithMetadata("data_truncated", result.DataTruncated)
	toolResult.WithMetadata("intent_type", result.IntentType)
	toolResult.WithMetadata("query_log_id", result.QueryLogID)

	// 如果有文件URL
	if result.FileURL != "" {
		toolResult.WithMetadata("file_url", result.FileURL)
	}

	// 添加文档引用
	if len(result.Documents) > 0 {
		for _, doc := range result.Documents {
			citation := schema.NewCitation(
				"nl2sql",
				"查询结果",
				"",
				doc.Content,
			)
			toolResult.WithCitation(citation)
		}
	}

	g.Log().Infof(ctx.Context, "[NL2SQLTool] Query completed: %d rows", result.RowCount)

	return toolResult, nil
}

// ParseConfig 从输入参数中解析配置并校验必要参数
func ParseConfig(input map[string]interface{}) (*NL2SQLConfig, error) {
	config := &NL2SQLConfig{}

	// 解析问题
	if question, ok := input["query"].(string); ok {
		config.Question = question
	}

	// 解析数据源ID
	if datasourceID, ok := input["datasource_id"].(string); ok {
		config.DatasourceID = datasourceID
	} else if datasourceID, ok := input["datasource"].(string); ok {
		config.DatasourceID = datasourceID
	}

	// 解析模型ID
	if modelID, ok := input["model_id"].(string); ok {
		config.ModelID = modelID
	}

	if config.Question == "" {
		return nil, fmt.Errorf("缺少必需参数 'query'")
	}
	if config.DatasourceID == "" {
		return nil, fmt.Errorf("缺少必需参数 'datasource_id'")
	}
	if config.ModelID == "" {
		return nil, fmt.Errorf("缺少必需参数 'model_id'")
	}

	return config, nil
}
