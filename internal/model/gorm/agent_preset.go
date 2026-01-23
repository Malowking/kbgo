package gorm

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentPreset Agent预设配置表
type AgentPreset struct {
	PresetID    string     `gorm:"primaryKey;column:preset_id;type:varchar(64)"`
	UserID      string     `gorm:"column:user_id;type:varchar(64);not null;index"` // 所属用户
	PresetName  string     `gorm:"column:preset_name;type:varchar(255);not null"`  // 预设名称（如"技术问答助手"）
	Description string     `gorm:"column:description;type:text"`                   // 预设描述
	Config      JSON       `gorm:"column:config;type:json;not null"`               // Agent配置（ChatReq参数，不包含Tools）
	Tools       JSON       `gorm:"column:tools;type:json"`                         // 工具配置（独立存储）
	IsPublic    bool       `gorm:"column:is_public;default:false"`                 // 是否公开分享
	CreateTime  *time.Time `gorm:"column:create_time;autoCreateTime"`              // 创建时间
	UpdateTime  *time.Time `gorm:"column:update_time;autoUpdateTime"`              // 更新时间
}

// TableName 设置表名
func (AgentPreset) TableName() string {
	return "agent_presets"
}

// BeforeCreate GORM钩子：创建前自动生成PresetID
func (a *AgentPreset) BeforeCreate(tx *gorm.DB) error {
	if a.PresetID == "" {
		// 生成格式：agent_uuid（无连接符）
		uuidStr := uuid.New().String()
		uuidStr = uuidStr[:8] + uuidStr[9:13] + uuidStr[14:18] + uuidStr[19:23] + uuidStr[24:]
		a.PresetID = fmt.Sprintf("agent_%s", uuidStr)
	}
	return nil
}
