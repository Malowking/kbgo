package gorm

import (
	"time"
)

// KnowledgeDocuments GORM模型定义
type KnowledgeDocuments struct {
	ID             string     `gorm:"primaryKey;column:id;varchar(255)" json:"id"`
	KnowledgeId    string     `gorm:"column:knowledge_id;type:varchar(255);not null" json:"knowledgeId"`
	FileName       string     `gorm:"column:file_name;type:varchar(255)" json:"fileName"`
	FileExtension  string     `gorm:"column:file_extension;type:varchar(255)" json:"fileExtension"` // 添加文件后缀名字段
	CollectionName string     `gorm:"column:collection_name;type:varchar(255)" json:"collectionName"`
	SHA256         string     `gorm:"column:sha256;type:varchar(64);index" json:"sha256"`
	RustfsBucket   string     `gorm:"column:rustfs_bucket;type:varchar(255)" json:"rustfsBucket"`
	RustfsLocation string     `gorm:"column:rustfs_location;type:varchar(255)" json:"rustfsLocation"`
	LocalFilePath  string     `gorm:"column:local_file_path;type:varchar(512)" json:"localFilePath"` // 本地文件路径
	Status         int8       `gorm:"column:status;not null;default:0" json:"status"`
	CreateTime     *time.Time `gorm:"column:create_time;type:timestamp;autoCreateTime" json:"CreateTime"`
	UpdateTime     *time.Time `gorm:"column:update_time;type:timestamp;autoUpdateTime" json:"UpdateTime"`
}

// TableName 设置表名
func (KnowledgeDocuments) TableName() string {
	return "knowledge_documents"
}
