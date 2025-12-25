package gorm

import (
	"time"
)

// KnowledgeBase GORM模型定义
type KnowledgeBase struct {
	ID               string     `gorm:"primaryKey;column:id;type:varchar(64)" json:"id"`
	Name             string     `gorm:"column:name;type:varchar(36)" json:"name"`
	Description      string     `gorm:"column:description;type:varchar(255)" json:"description"`
	Category         string     `gorm:"column:category;type:varchar(255)" json:"category"`
	CollectionName   string     `gorm:"column:collection_name;type:varchar(255)" json:"collectionName"`     // milvus collection name
	EmbeddingModelId string     `gorm:"column:embedding_model_id;type:varchar(64)" json:"embeddingModelId"` // embedding model id
	Status           int8       `gorm:"column:status;not null;default:1" json:"status"`
	CreateTime       *time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateTime       *time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
}

// TableName 设置表名
func (KnowledgeBase) TableName() string {
	return "knowledge_base"
}
