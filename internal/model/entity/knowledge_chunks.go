// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// KnowledgeChunks is the golang structure for table knowledge_chunks.
type KnowledgeChunks struct {
	Id             string      `json:"id"             orm:"id"               description:"chunk_id"` //
	KnowledgeDocId string      `json:"knowledgeDocId" orm:"knowledge_doc_id" description:""`         //
	Content        string      `json:"content"        orm:"content"          description:""`         //
	Ext            string      `json:"ext"            orm:"ext"              description:""`         //
	CollectionName string      `json:"collectionName" orm:"collection_name"  description:""`         // milvus collection name
	Status         int         `json:"status"         orm:"status"           description:""`         //
	CreateTime     *gtime.Time `json:"createTime"     orm:"create_time"      description:""`         //
	UpdateTime     *gtime.Time `json:"updateTime"     orm:"update_time"      description:""`         //
}
