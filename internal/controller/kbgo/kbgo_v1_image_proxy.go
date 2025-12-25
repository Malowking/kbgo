package kbgo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/gogf/gf/v2/frame/g"
)

// ImageProxy 图片代理接口
func (c *ControllerV1) ImageProxy(ctx context.Context, req *v1.ImageProxyReq) (res *v1.ImageProxyRes, err error) {
	r := g.RequestFromCtx(ctx)

	// 获取路径参数（从 /v1/images/* 中提取）
	path := r.Get("path").String()
	if path == "" {
		// 尝试从查询参数获取完整URL
		imageURL := r.Get("image_url").String()
		if imageURL != "" {
			// 从完整URL中提取路径
			// 例如: http://127.0.0.1:8002/images/xxx.jpeg -> xxx.jpeg
			parts := strings.Split(imageURL, "/images/")
			if len(parts) == 2 {
				path = parts[1]
			}
		}
	}

	if path == "" {
		r.Response.WriteStatus(http.StatusBadRequest)
		r.Response.WriteJson(g.Map{
			"error": "image path is required",
		})
		return
	}

	// 从配置文件获取 file_parse 服务地址
	fileParseURL := g.Cfg().MustGet(ctx, "fileParse.url", "http://localhost:8002").String()

	// 构建完整的图片URL
	imageURL := fmt.Sprintf("%s/images/%s", fileParseURL, path)
	g.Log().Infof(ctx, "Proxying image request to: %s", imageURL)

	// 从 file_parse 服务获取图片
	resp, err := http.Get(imageURL)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to fetch image from file_parse service: %v", err)
		r.Response.WriteStatus(http.StatusBadGateway)
		r.Response.WriteJson(g.Map{
			"error": fmt.Sprintf("Failed to fetch image: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		g.Log().Errorf(ctx, "file_parse service returned status: %d", resp.StatusCode)
		r.Response.WriteStatus(resp.StatusCode)
		r.Response.WriteJson(g.Map{
			"error": fmt.Sprintf("file_parse service error: status %d", resp.StatusCode),
		})
		return
	}

	// 设置响应头
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg" // 默认
	}
	r.Response.Header().Set("Content-Type", contentType)
	r.Response.Header().Set("Cache-Control", "public, max-age=86400") // 缓存1天

	// 将图片数据写入响应
	_, err = io.Copy(r.Response.Writer, resp.Body)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to write image data to response: %v", err)
		return
	}

	return
}
