package builtin

import (
	"github.com/Malowking/kbgo/core/agent_tools/registry"
)

// RegisterBuiltinTools 注册所有内置工具到工具定义注册表
func RegisterBuiltinTools(reg registry.ToolRegistry) error {
	// 注册 knowledge_retrieval
	if err := reg.RegisterTool("knowledge_retrieval", &registry.ToolDefinition{
		Name:        "knowledge_retrieval",
		Description: "从知识库中检索相关文档和信息。用于回答需要参考知识库内容的问题。",
		ToolType:    "local_tools",
		Parameters: &registry.ParameterSchema{
			Type: "object",
			Properties: map[string]*registry.PropertyDefinition{
				"query": {
					Type:        "string",
					Description: "检索查询语句，描述需要查找的信息",
				},
				"knowledge_id": {
					Type:        "string",
					Description: "知识库ID，指定要检索的知识库",
				},
				"top_k": {
					Type:        "integer",
					Description: "返回的文档数量，默认为5",
				},
				"score": {
					Type:        "number",
					Description: "相似度阈值，范围0-1，默认为0.7",
				},
				"retrieve_mode": {
					Type:        "string",
					Description: "检索模式：vector（向量检索）、keyword（关键词检索）、hybrid（混合检索）",
					Enum:        []string{"vector", "keyword", "hybrid"},
				},
				"enable_rewrite": {
					Type:        "boolean",
					Description: "是否启用查询重写，默认为true",
				},
				"rewrite_attempts": {
					Type:        "integer",
					Description: "查询重写尝试次数，默认为3",
				},
				"rerank_weight": {
					Type:        "number",
					Description: "重排序权重，范围0-1",
				},
				"rerank_model_id": {
					Type:        "string",
					Description: "重排序模型ID",
				},
			},
			Required: []string{"query"},
		},
	}); err != nil {
		return err
	}

	// 注册 nl2sql
	if err := reg.RegisterTool("nl2sql", &registry.ToolDefinition{
		Name:        "nl2sql",
		Description: "将自然语言问题转换为SQL查询并执行。用于查询结构化数据库。",
		ToolType:    "local_tools",
		Parameters: &registry.ParameterSchema{
			Type: "object",
			Properties: map[string]*registry.PropertyDefinition{
				"query": {
					Type:        "string",
					Description: "自然语言问题，描述需要查询的数据",
				},
				"datasource_id": {
					Type:        "string",
					Description: "数据源ID，指定要查询的数据库",
				},
				"model_id": {
					Type:        "string",
					Description: "用于生成SQL的模型ID",
				},
			},
			Required: []string{"query", "datasource_id", "model_id"},
		},
	}); err != nil {
		return err
	}

	// 注册 file_export
	if err := reg.RegisterTool("file_export", &registry.ToolDefinition{
		Name:        "file_export",
		Description: "将数据导出为文件（Excel、CSV等格式）。用于生成报表或数据导出。",
		ToolType:    "local_tools",
		Parameters: &registry.ParameterSchema{
			Type: "object",
			Properties: map[string]*registry.PropertyDefinition{
				"format": {
					Type:        "string",
					Description: "导出格式",
					Enum:        []string{"csv", "xlsx", "json", "md", "txt", "pdf"},
				},
				"filename": {
					Type:        "string",
					Description: "文件名（不含扩展名）",
				},
				"columns": {
					Type:        "array",
					Description: "列名数组，定义导出数据的列",
				},
				"data": {
					Type:        "array",
					Description: "要导出的数据，数组中每个元素是一个对象",
				},
				"title": {
					Type:        "string",
					Description: "文件标题（可选）",
				},
				"description": {
					Type:        "string",
					Description: "描述信息（可选）",
				},
				"base_dir": {
					Type:        "string",
					Description: "基础目录（可选，默认为 upload）",
				},
			},
			Required: []string{"format", "filename", "columns", "data"},
		},
	}); err != nil {
		return err
	}

	return nil
}
