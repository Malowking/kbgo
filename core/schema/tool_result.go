package schema

import (
	"time"
)

// ToolResult 统一的工具返回结果格式
// 用于承载工具执行后返回的各种类型数据
type ToolResult struct {
	Success   bool                   `json:"success"`   // 是否成功
	Data      interface{}            `json:"data"`      // 结果数据
	Error     *ToolError             `json:"error"`     // 错误信息
	Metadata  map[string]interface{} `json:"metadata"`  // 元数据
	Artifacts []*Artifact            `json:"artifacts"` // 附件（文件、图片等）
	Citations []*Citation            `json:"citations"` // 引用来源
	Metrics   *ExecutionMetrics      `json:"metrics"`   // 执行指标
}

// ToolError 工具错误信息
type ToolError struct {
	Code    string `json:"code"`    // 错误代码
	Message string `json:"message"` // 错误消息
	Details string `json:"details"` // 详细信息
}

// Artifact 附件信息
type Artifact struct {
	Type     string `json:"type"`      // 类型: "file", "image", "document"
	URL      string `json:"url"`       // 访问URL
	MimeType string `json:"mime_type"` // MIME类型
	Size     int64  `json:"size"`      // 文件大小
	Name     string `json:"name"`      // 文件名
}

// Citation 引用来源
type Citation struct {
	Source  string `json:"source"`  // 来源
	Title   string `json:"title"`   // 标题
	URL     string `json:"url"`     // URL
	Snippet string `json:"snippet"` // 摘要
}

// ExecutionMetrics 执行指标
type ExecutionMetrics struct {
	Duration   time.Duration `json:"duration"`    // 执行时长
	TokensUsed int           `json:"tokens_used"` // 使用的Token数
	Cost       float64       `json:"cost"`        // 成本
}

// NewToolResult 创建成功的工具结果
func NewToolResult(data interface{}) *ToolResult {
	return &ToolResult{
		Success:   true,
		Data:      data,
		Metadata:  make(map[string]interface{}),
		Artifacts: make([]*Artifact, 0),
		Citations: make([]*Citation, 0),
	}
}

// NewToolResultWithError 创建失败的工具结果
func NewToolResultWithError(code, message, details string) *ToolResult {
	return &ToolResult{
		Success: false,
		Error: &ToolError{
			Code:    code,
			Message: message,
			Details: details,
		},
		Metadata:  make(map[string]interface{}),
		Artifacts: make([]*Artifact, 0),
		Citations: make([]*Citation, 0),
	}
}

// WithMetadata 添加元数据
func (r *ToolResult) WithMetadata(key string, value interface{}) *ToolResult {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
	return r
}

// WithArtifact 添加附件
func (r *ToolResult) WithArtifact(artifact *Artifact) *ToolResult {
	if r.Artifacts == nil {
		r.Artifacts = make([]*Artifact, 0)
	}
	r.Artifacts = append(r.Artifacts, artifact)
	return r
}

// WithCitation 添加引用
func (r *ToolResult) WithCitation(citation *Citation) *ToolResult {
	if r.Citations == nil {
		r.Citations = make([]*Citation, 0)
	}
	r.Citations = append(r.Citations, citation)
	return r
}

// WithMetrics 设置执行指标
func (r *ToolResult) WithMetrics(metrics *ExecutionMetrics) *ToolResult {
	r.Metrics = metrics
	return r
}

// NewArtifact 创建附件
func NewArtifact(artifactType, url, mimeType string, size int64, name string) *Artifact {
	return &Artifact{
		Type:     artifactType,
		URL:      url,
		MimeType: mimeType,
		Size:     size,
		Name:     name,
	}
}

// NewCitation 创建引用
func NewCitation(source, title, url, snippet string) *Citation {
	return &Citation{
		Source:  source,
		Title:   title,
		URL:     url,
		Snippet: snippet,
	}
}

// NewExecutionMetrics 创建执行指标
func NewExecutionMetrics(duration time.Duration, tokensUsed int, cost float64) *ExecutionMetrics {
	return &ExecutionMetrics{
		Duration:   duration,
		TokensUsed: tokensUsed,
		Cost:       cost,
	}
}
