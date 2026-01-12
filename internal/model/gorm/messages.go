package gorm

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Message 消息表
type Message struct {
	MsgID      string     `gorm:"primaryKey;column:msg_id;type:varchar(64)"`      // 消息ID（主键，格式：uuid_timestamp）
	ConvID     string     `gorm:"column:conv_id;type:varchar(64);not null;index"` // 会话ID
	Role       string     `gorm:"column:role;type:varchar(20);not null"`          // 角色
	ToolCalls  JSON       `gorm:"column:tool_calls;type:json"`                    // 工具调用
	Skills     JSON       `gorm:"column:skills;type:json"`                        // Skill调用信息
	TokensUsed int        `gorm:"column:tokens_used;type:int"`                    // 使用的token数
	LatencyMs  int        `gorm:"column:latency_ms;type:int"`                     // 延迟毫秒数
	TraceID    string     `gorm:"column:trace_id;type:varchar(64)"`               // 链路追踪ID
	Metadata   JSON       `gorm:"column:metadata;type:json"`                      // 自定义扩展
	CreateTime *time.Time `gorm:"column:create_time;autoCreateTime"`              // 创建时间
}

// TableName 设置表名
func (Message) TableName() string {
	return "messages"
}

// BeforeCreate GORM钩子：创建前自动生成MsgID
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.MsgID == "" {
		// 生成格式：uuid（无连接符）_时间戳
		uuidStr := uuid.New().String()
		uuidStr = uuidStr[:8] + uuidStr[9:13] + uuidStr[14:18] + uuidStr[19:23] + uuidStr[24:]
		timestamp := time.Now().UnixMilli()
		m.MsgID = fmt.Sprintf("%s_%d", uuidStr, timestamp)
	}
	return nil
}
