package gorm

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
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

	// NL2SQL 配置字段（新增）
	NL2SQLEnabled         bool           `gorm:"column:nl2sql_enabled;default:false" json:"nl2sql_enabled"`
	NL2SQLConfig          datatypes.JSON `gorm:"column:nl2sql_config;type:jsonb" json:"nl2sql_config"`
	NL2SQLSchemaID        *string        `gorm:"column:nl2sql_schema_id;type:uuid" json:"nl2sql_schema_id"`
	NL2SQLStatus          string         `gorm:"column:nl2sql_status;size:50;default:'disabled'" json:"nl2sql_status"` // 'disabled', 'parsing', 'ready', 'error'
	NL2SQLLastSyncAt      *time.Time     `gorm:"column:nl2sql_last_sync_at" json:"nl2sql_last_sync_at"`
	NL2SQLError           string         `gorm:"column:nl2sql_error;type:text" json:"nl2sql_error"`
	NL2SQLEmbeddingModel  *string        `gorm:"column:nl2sql_embedding_model;size:255" json:"nl2sql_embedding_model"`      // Schema向量化使用的embedding模型
	NL2SQLKnowledgeBaseID *string        `gorm:"column:nl2sql_knowledge_base_id;type:uuid" json:"nl2sql_knowledge_base_id"` // 对应的知识库ID
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
