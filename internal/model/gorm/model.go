package gorm

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AIModel AI模型配置表（UUID作为主键，单表管理）
type AIModel struct {
	ModelID    string    `gorm:"primaryKey;type:char(36);column:model_id" json:"model_id"`       // UUID，唯一标识
	ModelType  string    `gorm:"type:varchar(50);not null;column:model_type" json:"model_type"`  // llm, embedding, reranker, multimodal, image, video, audio
	Provider   string    `gorm:"type:varchar(100);column:provider" json:"provider"`              // openai, groq, siliconflow...（可选）
	ModelName  string    `gorm:"type:varchar(200);not null;column:model_name" json:"model_name"` // 可读名字，如 GPT-4.1
	Version    string    `gorm:"type:varchar(50);column:version" json:"version"`                 // 可选，模型版本
	BaseURL    string    `gorm:"type:varchar(500);column:base_url" json:"base_url"`              // OpenAI-Compatible API Base URL（可选）
	APIKey     string    `gorm:"type:varchar(500);column:api_key" json:"api_key"`                // 模型调用 Key（可选）
	Extra      string    `gorm:"type:json;column:extra" json:"extra"`                            // 可扩展字段（JSON格式）
	Enabled    bool      `gorm:"type:tinyint(1);default:1;column:enabled" json:"enabled"`        // 是否启用
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

// TableName 指定表名
func (AIModel) TableName() string {
	return "model"
}

// BeforeCreate GORM钩子：创建前自动生成UUID
func (m *AIModel) BeforeCreate(tx *gorm.DB) error {
	if m.ModelID == "" {
		m.ModelID = uuid.New().String()
	}
	return nil
}
