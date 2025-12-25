package gorm

import (
	"time"
)

// KnowledgeChunks GORM模型定义
type KnowledgeChunks struct {
	ID             string     `gorm:"primaryKey;column:id;varchar(255)" json:"id"`
	KnowledgeDocID string     `gorm:"primaryKey;column:knowledge_doc_id;varchar(255);not null;index" json:"knowledgeDocId"`
	Content        string     `gorm:"column:content;type:text" json:"content"`
	CollectionName string     `gorm:"column:collection_name;type:varchar(255)" json:"collectionName"`
	Ext            string     `gorm:"column:ext;type:varchar(1024)" json:"ext"`
	Status         int8       `gorm:"column:status;not null;default:1" json:"status"`
	CreateTime     *time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"` // PostgreSQL timestamp
	UpdateTime     *time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"` // PostgreSQL timestamp

	KnowledgeDocument KnowledgeDocuments `gorm:"foreignKey:KnowledgeDocID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:RESTRICT" json:"-"`
}

// TableName 设置表名
func (KnowledgeChunks) TableName() string {
	return "knowledge_chunks"
}
