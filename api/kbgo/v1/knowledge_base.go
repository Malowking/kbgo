package v1

import (
	"github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// Status marks kb status.
type Status int

const (
	StatusOK       Status = 1
	StatusDisabled Status = 2
)

type KBCreateReq struct {
	g.Meta           `path:"/v1/kb" method:"post" tags:"kb" summary:"Create kb"`
	Name             string `v:"required|length:1,50" dc:"kb name"`
	Description      string `dc:"kb description"`
	Category         string `dc:"kb category"`
	EmbeddingModelId string `v:"required" dc:"embedding model id"`
}

type KBCreateRes struct {
	Id string `json:"id" dc:"kb id"`
}

type KBUpdateReq struct {
	g.Meta      `path:"/v1/kb/{id}" method:"put" tags:"kb" summary:"Update kb"`
	Id          string  `v:"required" dc:"kb id"`
	Name        *string `v:"length:1,50" dc:"kb name"`
	Description *string `dc:"kb description"`
	Category    *string `dc:"kb category"`
	Status      *Status `v:"in:1,2" dc:"kb status"`
}
type KBUpdateRes struct{}

type KBDeleteReq struct {
	g.Meta `path:"/v1/kb/{id}" method:"delete" tags:"kb" summary:"Delete kb"`
	Id     string `v:"required" dc:"kb id"`
}
type KBDeleteRes struct{}

type KBGetOneReq struct {
	g.Meta `path:"/v1/kb/{id}" method:"get" tags:"kb" summary:"Get one kb"`
	Id     string `v:"required" dc:"kb id"`
}
type KBGetOneRes struct {
	*gorm.KnowledgeBase `dc:"kb"`
}

type KBGetListReq struct {
	g.Meta   `path:"/v1/kb" method:"get" tags:"kb" summary:"Get kbs"`
	Name     *string `dc:"kb name"`
	Status   *Status `v:"in:1,2" dc:"kb age"`
	Category *string `dc:"kb category"`
}

type KBGetListRes struct {
	List []*gorm.KnowledgeBase `json:"list" dc:"kb list"`
}

type KBUpdateStatusReq struct {
	g.Meta `path:"/v1/kb/{id}/status" method:"patch" tags:"kb" summary:"Update kb status"`
	Id     string `v:"required" dc:"kb id"`
	Status Status `v:"required|in:1,2" dc:"kb status: 1-enabled, 2-disabled"`
}

type KBUpdateStatusRes struct{}
