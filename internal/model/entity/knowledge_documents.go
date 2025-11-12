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
	CollectionName string      `json:"collectionName"    orm:"collection_name"     description:""` //
	SHA256         string      `json:"sha256"            orm:"sha256"              description:""` //
	RustfsBucket   string      `json:"rustfsBucket"      orm:"rustfs_bucket"       description:""` //
	RustfsLocation string      `json:"rustfsLocation"    orm:"rustfs_location"     description:""` //
	IsQA           int         `json:"isQA"              orm:"is_qa"               description:""` //
	Status         int         `json:"status"            orm:"status"              description:""` //
	CreatedAt      *gtime.Time `json:"createdAt"         orm:"created_at"          description:""` //
	UpdatedAt      *gtime.Time `json:"updatedAt"         orm:"updated_at"          description:""` //
}
