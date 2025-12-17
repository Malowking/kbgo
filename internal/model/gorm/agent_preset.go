package gorm

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentPreset Agent预设配置表
type AgentPreset struct {
	ID          uint64     `gorm:"primaryKey;column:id;type:bigint"`
	PresetID    string     `gorm:"column:preset_id;type:varchar(64);uniqueIndex;not null"` // Agent预设ID
	UserID      string     `gorm:"column:user_id;type:varchar(64);not null;index"`         // 所属用户
	PresetName  string     `gorm:"column:preset_name;type:varchar(255);not null"`          // 预设名称（如"技术问答助手"）
	Description string     `gorm:"column:description;type:text"`                           // 预设描述
	Config      JSON       `gorm:"column:config;type:json;not null"`                       // Agent配置（包含所有ChatReq参数）
	IsPublic    bool       `gorm:"column:is_public;default:false"`                         // 是否公开分享
	CreateTime  *time.Time `gorm:"column:create_time;autoCreateTime"`                      // 创建时间
	UpdateTime  *time.Time `gorm:"column:update_time;autoUpdateTime"`                      // 更新时间
}

// TableName 设置表名
func (AgentPreset) TableName() string {
	return "agent_presets"
}

// BeforeCreate GORM钩子：创建前自动生成PresetID
func (a *AgentPreset) BeforeCreate(tx *gorm.DB) error {
	if a.PresetID == "" {
		a.PresetID = "preset_" + uuid.New().String()
	}
	return nil
}
