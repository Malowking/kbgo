package framework

// ToolDefinition 工具定义
type ToolDefinition struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`

	// InputSchema 输入参数 Schema（JSON Schema 格式）
	InputSchema map[string]interface{} `json:"input_schema"`

	// OutputSchema 输出参数 Schema（JSON Schema 格式）
	OutputSchema map[string]interface{} `json:"output_schema"`
}
