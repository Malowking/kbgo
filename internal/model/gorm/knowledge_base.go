package gorm

import (
	"time"
)

// KnowledgeBase GORM模型定义
type KnowledgeBase struct {
	ID             string     `gorm:"primaryKey;column:id;type:varchar(64)"`
	Name           string     `gorm:"column:name;type:varchar(36)"`
	Description    string     `gorm:"column:description;type:varchar(255)"`
	Category       string     `gorm:"column:category;type:varchar(255)"`
	CollectionName string     `gorm:"column:collection_name;type:varchar(255)"` // milvus collection name
	Status         int8       `gorm:"column:status;not null;default:1"`
	CreateTime     *time.Time `gorm:"column:create_time;autoCreateTime"`
	UpdateTime     *time.Time `gorm:"column:update_time;autoUpdateTime"`
}

// TableName 设置表名
func (KnowledgeBase) TableName() string {
	return "knowledge_base"
}
