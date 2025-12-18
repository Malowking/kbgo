package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// ImageProxyReq 图片代理请求
type ImageProxyReq struct {
	g.Meta   `path:"/v1/images/*path" method:"get" tags:"image"`
	Path     string `json:"path"`      // 图片路径
	ImageURL string `json:"image_url"` // 完整的图片URL
}

// ImageProxyRes 图片代理响应
type ImageProxyRes struct {
	g.Meta `mime:"image/*"`
	// 直接返回图片二进制数据
}
