// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// KnowledgeDocuments is the golang structure for table knowledge_documents.
type KnowledgeDocuments struct {
	Id             string      `json:"id"                orm:"id"                  description:""` //
	KnowledgeId    string      `json:"knowledgeId"       orm:"knowledge_id"        description:""` //
	FileName       string      `json:"fileName"          orm:"file_name"           description:""` //
	FileExtension  string      `json:"fileExtension"     orm:"file_extension"      description:""` // 添加文件后缀名字段
	CollectionName string      `json:"collectionName"    orm:"collection_name"     description:""` //
	SHA256         string      `json:"sha256"            orm:"sha256"              description:""` //
	RustfsBucket   string      `json:"rustfsBucket"      orm:"rustfs_bucket"       description:""` //
	RustfsLocation string      `json:"rustfsLocation"    orm:"rustfs_location"     description:""` //
	LocalFilePath  string      `json:"localFilePath"     orm:"local_file_path"     description:""` // 本地文件路径
	Status         int         `json:"status"            orm:"status"              description:""` //
	CreateTime     *gtime.Time `json:"CreateTime"        orm:"create_time"         description:""` //
	UpdateTime     *gtime.Time `json:"UpdateTime"        orm:"update_time"         description:""` //
}
