package gorm

import (
	"time"
)

// KnowledgeChunks GORM模型定义
type KnowledgeChunks struct {
	ID             string    `gorm:"primaryKey;column:id;varchar(255)"`
	KnowledgeDocID string    `gorm:"primaryKey;column:knowledge_doc_id;varchar(255);not null;index"`
	Content        string    `gorm:"column:content;type:text"`
	CollectionName string    `gorm:"column:collection_name;type:varchar(255)"`
	Ext            string    `gorm:"column:ext;type:varchar(1024)"`
	Status         int8      `gorm:"column:status;type:tinyint(1);not null;default:1"`
	CreateTime     time.Time `gorm:"column:created_at;type:datetime(3);not null;default:CURRENT_TIMESTAMP(3)"`
	UpdateTime     time.Time `gorm:"column:updated_at;type:datetime(3);default:CURRENT_TIMESTAMP(3) on update CURRENT_TIMESTAMP(3)"`

	KnowledgeDocument KnowledgeDocuments `gorm:"foreignKey:KnowledgeDocID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:RESTRICT"`
}

// TableName 设置表名
func (KnowledgeChunks) TableName() string {
	return "knowledge_chunks"
}
