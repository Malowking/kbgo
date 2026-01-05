package agent_tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent_tools/knowledge_retrieval"
	"github.com/Malowking/kbgo/core/agent_tools/mcp"
	"github.com/Malowking/kbgo/core/agent_tools/nl2sql"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ToolExecutor 统一的工具执行器
type ToolExecutor struct{}

// NewToolExecutor 创建工具执行器
func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{}
}

// ExecuteResult 工具执行结果
type ExecuteResult struct {
	Documents    []*schema.Document   // 工具返回的文档
	NL2SQLResult *v1.NL2SQLChatResult // NL2SQL结果
	MCPResults   []*v1.MCPResult      // MCP结果
	FinalAnswer  string               // 最终答案(如果工具直接返回)
	Error        string               // 错误信息
}

// Execute 执行所有配置的工具
// 这是chat_handler调用的唯一入口
func (e *ToolExecutor) Execute(ctx context.Context, tools []*v1.ToolConfig, question string, modelID string, embeddingModelID string, documents []*schema.Document) (*ExecuteResult, error) {
	result := &ExecuteResult{
		Documents: make([]*schema.Document, 0),
	}

	// 如果没有配置工具,直接返回
	if tools == nil || len(tools) == 0 {
		g.Log().Infof(ctx, "No tools configured")
		return result, nil
	}

	// 遍历所有工具配置
	for _, toolConfig := range tools {
		if !toolConfig.Enabled {
			g.Log().Infof(ctx, "Tool type '%s' is disabled, skipping", toolConfig.Type)
			continue
		}

		switch toolConfig.Type {
		case "local_tools":
			g.Log().Infof(ctx, "Executing local tools")
			err := e.executeLocalTools(ctx, toolConfig, question, modelID, embeddingModelID, result)
			if err != nil {
				g.Log().Errorf(ctx, "Failed to execute local tools: %v", err)
			}

		case "mcp":
			g.Log().Infof(ctx, "Executing MCP tools")
			err := e.executeMCPTools(ctx, toolConfig, question, modelID, documents, result)
			if err != nil {
				g.Log().Errorf(ctx, "Failed to execute MCP tools: %v", err)
			}

		default:
			g.Log().Warningf(ctx, "Unknown tool type: %s", toolConfig.Type)
		}

		// 如果有工具返回了最终答案,停止执行后续工具
		if result.FinalAnswer != "" {
			g.Log().Infof(ctx, "Tool returned final answer, stopping execution")
			break
		}
	}

	return result, nil
}

// executeLocalTools 执行本地工具
func (e *ToolExecutor) executeLocalTools(ctx context.Context, config *v1.ToolConfig, question string, modelID string, embeddingModelID string, result *ExecuteResult) error {
	if config.Config == nil {
		g.Log().Infof(ctx, "No local tools configuration")
		return nil
	}

	// 执行知识检索
	if krConfig, ok := config.Config["knowledge_retrieval"].(map[string]interface{}); ok {
		g.Log().Infof(ctx, "Executing knowledge retrieval tool")

		// 解析配置
		krToolConfig := knowledge_retrieval.ParseConfig(krConfig)

		// 执行知识检索
		krTool := knowledge_retrieval.NewKnowledgeRetrievalTool()
		krResult, err := krTool.Execute(ctx, krToolConfig, question)

		if err != nil {
			g.Log().Errorf(ctx, "Knowledge retrieval execution failed: %v", err)
			// 知识检索失败不应该阻止后续工具执行，只记录错误
		} else if len(krResult.Documents) > 0 {
			g.Log().Infof(ctx, "Knowledge retrieval retrieved %d documents", len(krResult.Documents))
			result.Documents = append(result.Documents, krResult.Documents...)
		}
	}

	// 执行NL2SQL
	if nl2sqlConfig, ok := config.Config["nl2sql"].(map[string]interface{}); ok {
		datasource, _ := nl2sqlConfig["datasource"].(string)
		if datasource != "" {
			g.Log().Infof(ctx, "Executing NL2SQL for datasource: %s", datasource)

			nl2sqlTool := nl2sql.NewNL2SQLTool()
			nl2sqlResult, err := nl2sqlTool.DetectAndExecute(ctx, question, datasource, modelID, embeddingModelID)

			if err != nil {
				g.Log().Errorf(ctx, "NL2SQL execution failed: %v", err)
				return err
			}

			if nl2sqlResult.IsNL2SQLQuery {
				if nl2sqlResult.Error == "" {
					// 成功执行
					g.Log().Infof(ctx, "NL2SQL executed successfully, %d rows returned", nl2sqlResult.RowCount)
					result.Documents = append(result.Documents, nl2sqlResult.Documents...)
					result.NL2SQLResult = &v1.NL2SQLChatResult{
						QueryLogID:      nl2sqlResult.QueryLogID,
						SQL:             nl2sqlResult.SQL,
						Columns:         nl2sqlResult.Columns,
						Data:            nl2sqlResult.Data,
						RowCount:        nl2sqlResult.RowCount,
						TotalRowCount:   nl2sqlResult.TotalRowCount,
						Explanation:     nl2sqlResult.Explanation,
						IntentType:      nl2sqlResult.IntentType,
						NeedLLMAnalysis: nl2sqlResult.NeedLLMAnalysis,
						AnalysisFocus:   nl2sqlResult.AnalysisFocus,
						DataTruncated:   nl2sqlResult.DataTruncated,
						FileURL:         nl2sqlResult.FileURL,
					}
				} else {
					// 执行失败
					g.Log().Warningf(ctx, "NL2SQL query failed: %s", nl2sqlResult.Error)
					result.NL2SQLResult = &v1.NL2SQLChatResult{
						QueryLogID:  nl2sqlResult.QueryLogID,
						SQL:         nl2sqlResult.SQL,
						Explanation: nl2sqlResult.Explanation,
						Error:       nl2sqlResult.Error,
					}
				}
			}
		}
	}

	// TODO: 添加其他本地工具的执行逻辑 (file_export等)

	return nil
}

// executeMCPTools 执行MCP工具
func (e *ToolExecutor) executeMCPTools(ctx context.Context, config *v1.ToolConfig, question string, modelID string, documents []*schema.Document, result *ExecuteResult) error {
	if config.Config == nil {
		g.Log().Infof(ctx, "No MCP tools configuration")
		return nil
	}

	// 从config中提取service_tools配置
	serviceTools, ok := config.Config["service_tools"].(map[string]interface{})
	if !ok || len(serviceTools) == 0 {
		g.Log().Infof(ctx, "No MCP service tools configured")
		return nil
	}

	// 转换为map[string][]string格式
	mcpServiceTools := make(map[string][]string)
	for service, tools := range serviceTools {
		if toolList, ok := tools.([]interface{}); ok {
			strTools := make([]string, 0, len(toolList))
			for _, tool := range toolList {
				if toolStr, ok := tool.(string); ok {
					strTools = append(strTools, toolStr)
				}
			}
			mcpServiceTools[service] = strTools
		}
	}

	g.Log().Infof(ctx, "Executing MCP tools with %d services", len(mcpServiceTools))

	// 创建 MCP 工具调用器
	toolCaller, err := mcp.NewMCPToolCaller(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to create MCP tool caller: %v", err)
		return errors.Newf(errors.ErrMCPInitFailed, "创建MCP工具调用器失败: %v", err)
	}
	defer toolCaller.Close()

	// 构建完整的用户问题（包含检索到的文档）
	fullQuestion := e.buildFullQuestion(ctx, question, documents)

	// 使用 LLM 智能选择并调用工具
	mcpDocuments, mcpResults, finalAnswer, err := toolCaller.CallToolsWithLLM(ctx, modelID, fullQuestion, "", mcpServiceTools)
	if err != nil {
		g.Log().Errorf(ctx, "MCP tools execution failed: %v", err)
		return errors.Newf(errors.ErrMCPCallFailed, "LLM intelligent tool call failed: %v", err)
	}

	// 收集MCP返回的文档
	if len(mcpDocuments) > 0 {
		result.Documents = append(result.Documents, mcpDocuments...)
	}

	// 收集MCP结果
	if len(mcpResults) > 0 {
		result.MCPResults = mcpResults
	}

	// 如果MCP返回了最终答案，设置到结果中
	if finalAnswer != "" {
		result.FinalAnswer = finalAnswer
		g.Log().Infof(ctx, "MCP tools returned final answer")
	}

	return nil
}

// buildFullQuestion 构建包含知识检索结果的完整问题
func (e *ToolExecutor) buildFullQuestion(ctx context.Context, question string, documents []*schema.Document) string {
	var builder strings.Builder

	builder.WriteString(question)

	// 如果有知识库检索内容，添加到问题中
	if len(documents) > 0 {
		builder.WriteString("\n\n【知识库检索结果】\n")
		for i, doc := range documents {
			builder.WriteString(fmt.Sprintf("文档%d: %s\n", i+1, doc.Content))
		}
	}

	return builder.String()
}
