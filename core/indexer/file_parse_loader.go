package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/gclient"
)

// FileParseLoader 使用 file_parse 服务进行文档解析和切分
type FileParseLoader struct {
	ctx            context.Context
	fileParseURL   string
	chunkSize      int
	chunkOverlap   int
	separators     []string
	imageURLFormat *bool // 是否格式化图片URL为静态地址，nil表示使用默认值
	client         *gclient.Client
}

// ParseRequest file_parse 服务的请求结构
type ParseRequest struct {
	FilePath       string   `json:"file_path"`
	ChunkSize      int      `json:"chunk_size"`
	ChunkOverlap   int      `json:"chunk_overlap"`
	Separators     []string `json:"separators"`
	ImageURLFormat *bool    `json:"image_url_format,omitempty"`
}

// ChunkData file_parse 服务返回的分片数据
type ChunkData struct {
	ChunkIndex int    `json:"chunk_index"`
	Text       string `json:"text"`
}

// ParseResponse file_parse 服务的响应结构
type ParseResponse struct {
	Success     bool        `json:"success"`
	Result      []ChunkData `json:"result"`
	ImageURLs   []string    `json:"image_urls"` // 顶层统一返回所有图片URL
	TotalChunks int         `json:"total_chunks"`
	TotalImages int         `json:"total_images"`
	FileInfo    interface{} `json:"file_info"`
}

// HealthResponse file_parse 服务健康检查响应结构
type HealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Version string `json:"version"`
}

// NewFileParseLoader 创建新的 FileParseLoader
func NewFileParseLoader(ctx context.Context, chunkSize, chunkOverlap int, separator string) (*FileParseLoader, error) {
	// 从配置中读取 file_parse 服务地址和超时时间
	fileParseURL := g.Cfg().MustGet(ctx, "fileParse.url", "http://localhost:8002").String()
	timeout := g.Cfg().MustGet(ctx, "fileParse.timeout", 600).Int() // 增加默认超时时间到600秒(10分钟)

	// 处理分隔符 - 必须是数组，不能为 nil
	var separators []string
	if separator != "" {
		separators = []string{separator}
	} else {
		// 如果没有指定分隔符，使用空数组而不是 nil
		separators = []string{}
	}

	// 如果未指定，使用默认值
	if chunkSize == 0 {
		chunkSize = 1000
	}
	if chunkOverlap == 0 {
		chunkOverlap = 200
	}

	// 使用 gf 的轻量级 HTTP 客户端，配置长连接和超时
	client := g.Client()
	client.SetTimeout(time.Duration(timeout) * time.Second) // 整体请求超时时间

	// 配置底层HTTP Transport以支持长时间连接和大文件传输
	client.Transport = &http.Transport{
		MaxIdleConns:        100,              // 最大空闲连接数
		MaxIdleConnsPerHost: 10,               // 每个host的最大空闲连接数
		IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间
		DisableKeepAlives:   false,            // 启用 Keep-Alive
		DisableCompression:  false,            // 启用压缩
		// 增加读写超时时间，避免大文件传输时超时
		ResponseHeaderTimeout: time.Duration(timeout) * time.Second, // 等待响应头超时时间
		// ExpectContinueTimeout: 1 * time.Second, // 如果服务器支持100-continue
	}

	return &FileParseLoader{
		ctx:          ctx,
		fileParseURL: fileParseURL,
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
		separators:   separators,
		client:       client,
	}, nil
}

// NewFileParseLoaderForChat 创建用于文件对话的 FileParseLoader，imageURLFormat=false返回绝对路径
func NewFileParseLoaderForChat(ctx context.Context, chunkSize, chunkOverlap int, separator string) (*FileParseLoader, error) {
	loader, err := NewFileParseLoader(ctx, chunkSize, chunkOverlap, separator)
	if err != nil {
		return nil, err
	}

	// 设置 imageURLFormat 为 false，返回绝对路径而不是HTTP URL
	imageURLFormat := false
	loader.imageURLFormat = &imageURLFormat

	return loader, nil
}

// CheckHealth 检查 file_parse 服务健康状态
func (l *FileParseLoader) CheckHealth(ctx context.Context) error {
	healthURL := fmt.Sprintf("%s/health", l.fileParseURL)

	resp, err := l.client.Get(ctx, healthURL)
	if err != nil {
		return fmt.Errorf("file_parse server is not running or unreachable: %w", err)
	}
	defer resp.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("file_parse server health check failed with status %d", resp.StatusCode)
	}

	// 解析健康检查响应
	var healthResp HealthResponse
	if err := json.Unmarshal(resp.ReadAll(), &healthResp); err != nil {
		return fmt.Errorf("failed to unmarshal health check response: %w", err)
	}

	if healthResp.Status != "healthy" {
		return fmt.Errorf("file_parse server is not healthy: status=%s", healthResp.Status)
	}

	g.Log().Infof(ctx, "file_parse server is healthy: %s (version: %s)", healthResp.Message, healthResp.Version)
	return nil
}

// Load 加载并解析文档，调用 file_parse 服务
func (l *FileParseLoader) Load(ctx context.Context, filePath string) ([]*schema.Document, error) {
	g.Log().Infof(ctx, "Starting to parse file using file_parse service: %s", filePath)

	// 首先检查服务健康状态
	if err := l.CheckHealth(ctx); err != nil {
		g.Log().Errorf(ctx, "file_parse server health check failed: %v", err)
		return nil, fmt.Errorf("file_parse server is not running: %w", err)
	}

	// 确保文件路径是绝对路径
	absFilePath := filePath
	if !filepath.IsAbs(filePath) {
		// 获取当前工作目录
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		g.Log().Debugf(ctx, "Current working directory: %s", cwd)

		// 基于当前工作目录构建绝对路径
		absFilePath = filepath.Join(cwd, filePath)
	}

	// 检查文件是否存在
	if _, err := os.Stat(absFilePath); os.IsNotExist(err) {
		g.Log().Errorf(ctx, "File does not exist at path: %s", absFilePath)
		return nil, fmt.Errorf("file does not exist: %s", absFilePath)
	}

	g.Log().Infof(ctx, "File exists, ready to parse: %s", absFilePath)

	// 构造请求
	parseReq := ParseRequest{
		FilePath:       absFilePath,
		ChunkSize:      l.chunkSize,
		ChunkOverlap:   l.chunkOverlap,
		Separators:     l.separators,     // 现在保证是数组，不会是 nil
		ImageURLFormat: l.imageURLFormat, // 传递imageURLFormat参数
	}

	// 记录开始时间
	startTime := time.Now()

	// 发送 HTTP 请求到 file_parse 服务
	parseURL := fmt.Sprintf("%s/parse", l.fileParseURL)
	g.Log().Infof(ctx, "Calling file_parse service: %s with params: chunkSize=%d, chunkOverlap=%d, separators=%v",
		parseURL, parseReq.ChunkSize, parseReq.ChunkOverlap, parseReq.Separators)

	// 使用 gf 的 HTTP 客户端发送 POST 请求
	resp, err := l.client.ContentJson().Post(ctx, parseURL, parseReq)
	if err != nil {
		// 检查是否是超时错误
		if os.IsTimeout(err) {
			return nil, fmt.Errorf("file_parse request timeout after %v: %w", time.Since(startTime), err)
		}
		return nil, fmt.Errorf("failed to call file_parse service: %w", err)
	}
	defer resp.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		body := resp.ReadAllString()
		g.Log().Errorf(ctx, "file_parse service error response: %s", body)
		return nil, fmt.Errorf("file_parse service returned error status %d: %s", resp.StatusCode, body)
	}

	// 解析响应
	var parseResp ParseResponse
	if err := json.Unmarshal(resp.ReadAll(), &parseResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parse response: %w", err)
	}

	if !parseResp.Success {
		return nil, fmt.Errorf("file_parse service returned success=false")
	}

	g.Log().Infof(ctx, "File parsed successfully: %d chunks, %d images (took %v)", parseResp.TotalChunks, parseResp.TotalImages, time.Since(startTime))

	// 转换为 schema.Document
	documents := make([]*schema.Document, len(parseResp.Result))
	for i, chunk := range parseResp.Result {
		metadata := map[string]interface{}{
			"chunk_index": chunk.ChunkIndex,
		}

		// 如果是chunk_size=-1的情况，所有图片在顶层ImageURLs中
		// 将顶层的ImageURLs添加到第一个document的metadata中
		if l.chunkSize == -1 && i == 0 && len(parseResp.ImageURLs) > 0 {
			metadata["image_urls"] = parseResp.ImageURLs
		}

		documents[i] = &schema.Document{
			Content:  chunk.Text,
			MetaData: metadata,
		}
	}

	g.Log().Infof(ctx, "Converted %d chunks to documents", len(documents))
	return documents, nil
}
