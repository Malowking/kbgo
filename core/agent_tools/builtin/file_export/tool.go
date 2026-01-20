package file_export

import (
	"encoding/json"
	"fmt"

	"github.com/Malowking/kbgo/core/agent_tools/framework"
	"github.com/Malowking/kbgo/core/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// FileExportConfig 文件导出配置
type FileExportConfig struct {
	Format      string                   `json:"format"`      // 导出格式: csv, xlsx, json, md, txt, pdf
	Filename    string                   `json:"filename"`    // 文件名（不含扩展名）
	Columns     []string                 `json:"columns"`     // 列名
	Data        []map[string]interface{} `json:"data"`        // 数据
	Title       string                   `json:"title"`       // 文件标题（可选）
	Description string                   `json:"description"` // 描述信息（可选）
	BaseDir     string                   `json:"base_dir"`    // 基础目录（可选，默认为 upload）
}

// FileExportTool 文件导出工具
type FileExportTool struct {
	baseDir string
}

// NewFileExportTool 创建文件导出工具
func NewFileExportTool() *FileExportTool {
	return &FileExportTool{
		baseDir: "upload", // 默认使用 upload 目录
	}
}

// Execute 执行文件导出
func (t *FileExportTool) Execute(
	ctx framework.ToolContext,
	config *FileExportConfig,
	params map[string]interface{},
) (*schema.ToolResult, error) {
	g.Log().Infof(ctx.Context, "[FileExportTool] Starting file export, format: %s", config.Format)

	// 如果配置中指定了 base_dir，使用它
	baseDir := t.baseDir
	if config.BaseDir != "" {
		baseDir = config.BaseDir
	}

	// 创建导出器
	exporter := NewFileExporter(baseDir)

	// 构建导出请求
	exportReq := &ExportRequest{
		Format:      ExportFormat(config.Format),
		Filename:    config.Filename,
		Columns:     config.Columns,
		Data:        config.Data,
		Title:       config.Title,
		Description: config.Description,
	}

	// 执行导出
	result, err := exporter.Export(ctx.Context, exportReq)
	if err != nil {
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("文件导出失败: %v", err), ""),
		), nil
	}

	// 构建返回内容
	content := fmt.Sprintf("文件导出成功\n文件名: %s\n格式: %s\n大小: %d 字节\n行数: %d\n下载地址: %s",
		result.Filename,
		result.Format,
		result.Size,
		result.RowCount,
		result.FileURL,
	)

	toolResult := schema.NewToolResult(content)

	// 添加元数据
	toolResult.WithMetadata("file_path", result.FilePath)
	toolResult.WithMetadata("file_url", result.FileURL)
	toolResult.WithMetadata("filename", result.Filename)
	toolResult.WithMetadata("format", result.Format)
	toolResult.WithMetadata("size", result.Size)
	toolResult.WithMetadata("row_count", result.RowCount)
	toolResult.WithMetadata("generated_at", result.GeneratedAt)

	// 添加文档引用（包含下载链接）
	citation := schema.NewCitation(
		"file_export",
		result.Filename,
		result.FileURL,
		content,
	)
	toolResult.WithCitation(citation)

	g.Log().Infof(ctx.Context, "[FileExportTool] Export completed: %s", result.Filename)

	return toolResult, nil
}

// ParseConfig 从输入参数中解析配置
func ParseConfig(input map[string]interface{}) (*FileExportConfig, error) {
	config := &FileExportConfig{}

	// 解析格式
	if format, ok := input["format"].(string); ok {
		config.Format = format
	} else {
		return nil, fmt.Errorf("format is required")
	}

	// 解析文件名
	if filename, ok := input["filename"].(string); ok {
		config.Filename = filename
	} else {
		return nil, fmt.Errorf("filename is required")
	}

	// 解析列名
	if columns, ok := input["columns"].([]interface{}); ok {
		config.Columns = make([]string, len(columns))
		for i, col := range columns {
			if colStr, ok := col.(string); ok {
				config.Columns[i] = colStr
			}
		}
	} else if columnsJSON, ok := input["columns"].(string); ok {
		// 如果是 JSON 字符串，尝试解析
		if err := json.Unmarshal([]byte(columnsJSON), &config.Columns); err != nil {
			return nil, fmt.Errorf("failed to parse columns: %w", err)
		}
	} else {
		return nil, fmt.Errorf("columns are required")
	}

	// 解析数据
	if data, ok := input["data"].([]interface{}); ok {
		config.Data = make([]map[string]interface{}, len(data))
		for i, row := range data {
			if rowMap, ok := row.(map[string]interface{}); ok {
				config.Data[i] = rowMap
			}
		}
	} else if dataJSON, ok := input["data"].(string); ok {
		// 如果是 JSON 字符串，尝试解析
		if err := json.Unmarshal([]byte(dataJSON), &config.Data); err != nil {
			return nil, fmt.Errorf("failed to parse data: %w", err)
		}
	} else {
		return nil, fmt.Errorf("data is required")
	}

	// 可选参数
	if title, ok := input["title"].(string); ok {
		config.Title = title
	}
	if description, ok := input["description"].(string); ok {
		config.Description = description
	}
	if baseDir, ok := input["base_dir"].(string); ok {
		config.BaseDir = baseDir
	}

	return config, nil
}
