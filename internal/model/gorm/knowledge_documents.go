package gorm

import (
	"time"
)

// KnowledgeDocuments GORM模型定义
type KnowledgeDocuments struct {
	ID             string     `gorm:"primaryKey;column:id;varchar(255)"`
	KnowledgeId    string     `gorm:"column:knowledge_id;type:varchar(255);not null"`
	FileName       string     `gorm:"column:file_name;type:varchar(255)"`
	CollectionName string     `gorm:"column:collection_name;type:varchar(255)"`
	SHA256         string     `gorm:"column:sha256;type:varchar(64);index"`
	RustfsBucket   string     `gorm:"column:rustfs_bucket;type:varchar(255)"`
	RustfsLocation string     `gorm:"column:rustfs_location;type:varchar(255)"`
	LocalFilePath  string     `gorm:"column:local_file_path;type:varchar(512)"` // 本地文件路径
	IsQA           int8       `gorm:"column:is_qa;type:tinyint;not null;default:0"`
	Status         int8       `gorm:"column:status;type:tinyint;not null;default:0"`
	CreateTime     *time.Time `gorm:"column:create_time;type:timestamp;autoCreateTime"`
	UpdateTime     *time.Time `gorm:"column:update_time;type:timestamp;autoUpdateTime"`
}

// TableName 设置表名
func (KnowledgeDocuments) TableName() string {
	return "knowledge_documents"
}
