package gorm

import (
	"time"
)

// MCPCallLog MCP调用日志表 GORM模型定义
type MCPCallLog struct {
	ID              string     `gorm:"primaryKey;column:id;type:varchar(64)"`                   // 主键ID
	ConversationID  string     `gorm:"column:conversation_id;type:varchar(255);index;not null"` // 对话ID（关联外部对话历史）
	MCPRegistryID   string     `gorm:"column:mcp_registry_id;type:varchar(64);index"`           // MCP服务ID（外键）
	MCPServiceName  string     `gorm:"column:mcp_service_name;type:varchar(100)"`               // MCP服务名称快照
	ToolName        string     `gorm:"column:tool_name;type:varchar(100)"`                      // 调用的工具名称
	RequestPayload  string     `gorm:"column:request_payload;type:text"`                        // 请求参数（JSON）
	ResponsePayload string     `gorm:"column:response_payload;type:text"`                       // 响应结果（JSON）
	Status          int8       `gorm:"column:status;default:1"`                                 // 状态：1-成功，0-失败，2-超时
	ErrorMessage    string     `gorm:"column:error_message;type:text"`                          // 错误信息
	Duration        int        `gorm:"column:duration;default:0"`                               // 调用耗时（毫秒）
	CreateTime      *time.Time `gorm:"column:create_time;autoCreateTime"`                       // 创建时间
}

// TableName 设置表名
func (MCPCallLog) TableName() string {
	return "mcp_call_log"
}
