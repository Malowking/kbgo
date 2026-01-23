package agent_tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/core/agent_tools/builtin/file_export"
	"github.com/Malowking/kbgo/core/agent_tools/builtin/knowledge_retrieval"
	"github.com/Malowking/kbgo/core/agent_tools/builtin/nl2sql"
	"github.com/Malowking/kbgo/core/agent_tools/executor/mcp"
	"github.com/Malowking/kbgo/core/agent_tools/framework"
	"github.com/Malowking/kbgo/core/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ToolManager 工具管理器
type ToolManager struct {
	// MCP 工具调用器
	mcpToolCaller *mcp.MCPToolCaller

	// 内置工具实例
	knowledgeRetrievalTool *knowledge_retrieval.KnowledgeRetrievalToolAdapter
	nl2sqlTool             *nl2sql.NL2SQLToolAdapter
	fileExportTool         *file_export.FileExportTool
}

// NewToolManager 创建工具管理器
func NewToolManager() *ToolManager {
	// 创建内置工具实例
	knowledgeRetrievalTool := knowledge_retrieval.NewKnowledgeRetrievalToolAdapter()
	nl2sqlTool := nl2sql.NewNL2SQLToolAdapter()
	fileExportTool := file_export.NewFileExportTool()

	tm := &ToolManager{
		knowledgeRetrievalTool: knowledgeRetrievalTool,
		nl2sqlTool:             nl2sqlTool,
		fileExportTool:         fileExportTool,
	}

	return tm
}

// SetMCPExecutor 设置MCP执行器
func (tm *ToolManager) SetMCPExecutor(ctx context.Context) (*mcp.MCPToolCaller, error) {
	// 创建 MCP 工具调用器
	mcpToolCaller, err := mcp.NewMCPToolCaller(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP tool caller: %w", err)
	}

	// 保存 MCP 工具调用器
	tm.mcpToolCaller = mcpToolCaller

	g.Log().Infof(ctx, "[ToolManager] MCP工具调用器已创建")
	return mcpToolCaller, nil
}

// ExecuteTool 执行工具（统一入口）
func (tm *ToolManager) ExecuteTool(
	ctx context.Context,
	toolName string,
	params map[string]interface{},
) (*schema.ToolResult, error) {
	g.Log().Infof(ctx, "[ToolManager] Executing tool: %s, params: %+v", toolName, params)

	// 构建工具上下文
	toolCtx := framework.ToolContext{
		Context:   ctx,
		SessionID: "", // 可以从 ctx 中提取
	}

	// 判断工具类型并执行
	if strings.HasPrefix(toolName, "mcp_") {
		// MCP 工具
		return tm.executeMCPTool(toolCtx, toolName, params)
	}

	// 内置工具
	return tm.executeBuiltinTool(toolCtx, toolName, params)
}

// executeBuiltinTool 执行内置工具
func (tm *ToolManager) executeBuiltinTool(
	ctx framework.ToolContext,
	toolName string,
	params map[string]interface{},
) (*schema.ToolResult, error) {
	switch toolName {
	case "knowledge_retrieval":
		return tm.executeKnowledgeRetrieval(ctx, params)
	case "nl2sql":
		return tm.executeNL2SQL(ctx, params)
	case "file_export":
		return tm.executeFileExport(ctx, params)
	default:
		return schema.NewToolResultFromError(
			schema.NewNotFoundError(fmt.Sprintf("未知的工具: %s", toolName), ""),
		), nil
	}
}

// executeKnowledgeRetrieval 执行知识检索
func (tm *ToolManager) executeKnowledgeRetrieval(
	ctx framework.ToolContext,
	params map[string]interface{},
) (*schema.ToolResult, error) {
	// 解析配置
	config, err := knowledge_retrieval.ParseConfig(params)
	if err != nil {
		g.Log().Errorf(ctx.Context, "[ToolManager] 知识检索参数解析失败: %v, params: %+v", err, params)
		return schema.NewToolResultFromError(
			schema.NewValidationError(fmt.Sprintf("知识检索参数解析失败: %v", err), ""),
		), nil
	}

	// 执行工具
	result, err := tm.knowledgeRetrievalTool.Execute(ctx, config, params)
	if err != nil {
		g.Log().Errorf(ctx.Context, "[ToolManager] 知识检索执行失败: %v", err)
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("知识检索执行失败: %v", err), ""),
		), nil
	}

	return result, nil
}

// executeNL2SQL 执行 NL2SQL
func (tm *ToolManager) executeNL2SQL(
	ctx framework.ToolContext,
	params map[string]interface{},
) (*schema.ToolResult, error) {
	// 解析配置
	config, err := nl2sql.ParseConfig(params)
	if err != nil {
		g.Log().Errorf(ctx.Context, "[ToolManager] NL2SQL参数解析失败: %v, params: %+v", err, params)
		return schema.NewToolResultFromError(
			schema.NewValidationError(fmt.Sprintf("NL2SQL参数解析失败: %v", err), ""),
		), nil
	}

	// 执行工具
	result, err := tm.nl2sqlTool.Execute(ctx, config, params)
	if err != nil {
		g.Log().Errorf(ctx.Context, "[ToolManager] NL2SQL执行失败: %v", err)
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("NL2SQL执行失败: %v", err), ""),
		), nil
	}

	return result, nil
}

// executeFileExport 执行文件导出
func (tm *ToolManager) executeFileExport(
	ctx framework.ToolContext,
	params map[string]interface{},
) (*schema.ToolResult, error) {
	// 解析配置
	config, err := file_export.ParseConfig(params)
	if err != nil {
		g.Log().Errorf(ctx.Context, "[ToolManager] 文件导出参数解析失败: %v, params: %+v", err, params)
		return schema.NewToolResultFromError(
			schema.NewValidationError(fmt.Sprintf("文件导出参数解析失败: %v", err), ""),
		), nil
	}

	// 执行工具
	result, err := tm.fileExportTool.Execute(ctx, config, params)
	if err != nil {
		g.Log().Errorf(ctx.Context, "[ToolManager] 文件导出执行失败: %v", err)
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("文件导出执行失败: %v", err), ""),
		), nil
	}

	return result, nil
}

// executeMCPTool 执行 MCP 工具
func (tm *ToolManager) executeMCPTool(
	ctx framework.ToolContext,
	toolName string,
	params map[string]interface{},
) (*schema.ToolResult, error) {
	if tm.mcpToolCaller == nil {
		return schema.NewToolResultFromError(
			schema.NewExecutionError("MCP工具调用器未初始化", ""),
		), nil
	}

	// 构建工具定义（MCPToolCaller.Execute 需要 framework.ToolDefinition）
	toolDef := framework.ToolDefinition{
		Name: toolName,
	}

	// 调用 MCP 工具
	result, err := tm.mcpToolCaller.Execute(ctx, toolDef, params)
	if err != nil {
		g.Log().Errorf(ctx.Context, "[ToolManager] MCP工具执行失败: %v", err)
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("MCP工具执行失败: %v", err), ""),
		), nil
	}

	return result, nil
}

// GetMCPExecutor 获取 MCP 执行器（用于访问 MCP 客户端）
func (tm *ToolManager) GetMCPExecutor() *mcp.MCPToolCaller {
	return tm.mcpToolCaller
}
