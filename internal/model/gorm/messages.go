package gorm

import (
	"time"
)

// Message 消息表
type Message struct {
	ID         uint64     `gorm:"primaryKey;column:id;type:bigint"`
	MsgID      string     `gorm:"column:msg_id;type:varchar(64);uniqueIndex;not null"` // 消息ID
	ConvID     string     `gorm:"column:conv_id;type:varchar(64);not null;index"`      // 会话ID
	Role       string     `gorm:"column:role;type:varchar(20);not null"`               // 角色
	ToolCalls  JSON       `gorm:"column:tool_calls;type:json"`                         // 工具调用
	ToolCallID string     `gorm:"column:tool_call_id;type:varchar(64)"`                // 工具调用ID
	ToolName   string     `gorm:"column:tool_name;type:varchar(128)"`                  // 工具名称
	TokensUsed int        `gorm:"column:tokens_used;type:int"`                         // 使用的token数
	LatencyMs  int        `gorm:"column:latency_ms;type:int"`                          // 延迟毫秒数
	TraceID    string     `gorm:"column:trace_id;type:varchar(64)"`                    // 链路追踪ID
	Metadata   JSON       `gorm:"column:metadata;type:json"`                           // 自定义扩展
	CreateTime *time.Time `gorm:"column:create_time"`                                  // 创建时间
}

// TableName 设置表名
func (Message) TableName() string {
	return "messages"
}
