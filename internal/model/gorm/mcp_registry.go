package gorm

import (
	"time"
)

// MCPRegistry MCP服务注册表 GORM模型定义
type MCPRegistry struct {
	ID          string     `gorm:"primaryKey;column:id;type:varchar(64)"`              // MCP服务唯一ID
	Name        string     `gorm:"column:name;type:varchar(100);not null;uniqueIndex"` // MCP服务名称（唯一）
	Description string     `gorm:"column:description;type:varchar(500)"`               // 服务描述
	Endpoint    string     `gorm:"column:endpoint;type:varchar(500);not null"`         // SSE端点URL
	ApiKey      string     `gorm:"column:api_key;type:varchar(500)"`                   // 认证密钥（加密存储）
	Headers     string     `gorm:"column:headers;type:text"`                           // 自定义请求头（JSON格式）
	Timeout     int        `gorm:"column:timeout;default:30"`                          // 超时时间（秒）
	Status      int8       `gorm:"column:status;default:1"`                            // 状态：1-启用，0-禁用
	Tools       string     `gorm:"column:tools;type:text"`                             // 工具列表（JSON格式存储）
	CreateTime  *time.Time `gorm:"column:create_time;autoCreateTime"`                  // 创建时间
	UpdateTime  *time.Time `gorm:"column:update_time;autoUpdateTime"`                  // 更新时间
}

// TableName 设置表名
func (MCPRegistry) TableName() string {
	return "mcp_registry"
}
