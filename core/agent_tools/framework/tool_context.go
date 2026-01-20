package framework

import (
	"context"

	"github.com/Malowking/kbgo/core/schema"
)

// ToolContext 执行期上下文
type ToolContext struct {
	// Context Go 标准上下文
	Context context.Context

	// SessionID 会话ID
	SessionID string

	// UserID 用户ID
	UserID string

	// AgentID Agent ID
	AgentID string

	// Logger 日志记录器（可选）
	Logger Logger

	// Tracer 追踪器（可选）
	Tracer Tracer

	// CallTool 工具调用函数（用于工具间相互调用）
	CallTool func(name string, input map[string]interface{}) (*schema.ToolResult, error)
}

// Logger 日志接口
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// Tracer 追踪接口
type Tracer interface {
	StartSpan(name string) Span
}

// Span 追踪 Span
type Span interface {
	End()
	SetAttribute(key string, value interface{})
}
