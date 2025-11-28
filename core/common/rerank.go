package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// RerankConfig 接口，用于提取rerank配置
type RerankConfig interface {
	GetRerankAPIKey() string
	GetRerankBaseURL() string
	GetRerankModel() string
}

// CustomReranker 自定义rerank客户端
type CustomReranker struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// RerankDocument 简化的文档结构
type RerankDocument struct {
	ID      string
	Content string
	Score   float64
}

// RerankRequest rerank API请求结构
type RerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int      `json:"top_n"`
	ReturnDocuments bool     `json:"return_documents"`
	MaxChunksPerDoc int      `json:"max_chunks_per_doc,omitempty"`
	OverlapTokens   int      `json:"overlap_tokens,omitempty"`
}

// RerankResult rerank结果项
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// RerankResponse rerank API响应结构
type RerankResponse struct {
	ID      string          `json:"id"`
	Results []*RerankResult `json:"results"`
}

// RerankErrorResponse API错误响应
type RerankErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// NewReranker 创建rerank客户端
func NewReranker(ctx context.Context, conf RerankConfig) (*CustomReranker, error) {
	apiKey := conf.GetRerankAPIKey()
	baseURL := conf.GetRerankBaseURL()
	model := conf.GetRerankModel()

	if apiKey == "" {
		apiKey = os.Getenv("RERANK_API_KEY")
	}
	if baseURL == "" {
		baseURL = os.Getenv("RERANK_BASE_URL")
		if baseURL == "" {
			return nil, fmt.Errorf("rerank baseURL is required")
		}
	}
	if model == "" {
		model = "rerank-v1"
	}

	// 创建自定义HTTP客户端，优化连接复用和超时
	httpClient := &http.Client{
		Timeout: 2 * time.Minute, // rerank 通常比 embedding 快
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second, // 连接超时
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   30 * time.Second, // TLS握手超时
			ResponseHeaderTimeout: 60 * time.Second, // 等待响应头超时
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20, // 增加每个host的连接数，支持并发
		},
	}

	return &CustomReranker{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		httpClient: httpClient,
	}, nil
}

// Rerank 执行重排序
func (r *CustomReranker) Rerank(ctx context.Context, query string, docs []RerankDocument, topK int) ([]RerankDocument, error) {
	if len(docs) == 0 {
		return []RerankDocument{}, nil
	}

	// 如果文档数量少于等于topK，仍然需要rerank来获取相关性分数
	if topK <= 0 {
		topK = len(docs)
	}
	if topK > len(docs) {
		topK = len(docs)
	}

	// 提取文档内容
	documents := make([]string, len(docs))
	for i, doc := range docs {
		documents[i] = doc.Content
	}

	// 构造请求
	req := RerankRequest{
		Model:           r.model,
		Query:           query,
		Documents:       documents,
		TopN:            topK,
		ReturnDocuments: false,
		MaxChunksPerDoc: 1024,
		OverlapTokens:   80,
	}

	// 序列化请求
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建HTTP请求
	url := r.baseURL + "/rerank"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)

	// 发送请求
	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		var errResp RerankErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("HTTP %d: failed to decode error response: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
	}

	// 解析响应
	var rerankResp RerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&rerankResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 验证响应数据
	if len(rerankResp.Results) == 0 {
		return []RerankDocument{}, nil
	}

	// 构造返回结果
	result := make([]RerankDocument, 0, len(rerankResp.Results))
	for _, res := range rerankResp.Results {
		if res.Index >= len(docs) {
			return nil, fmt.Errorf("invalid result index: %d", res.Index)
		}
		doc := docs[res.Index]
		doc.Score = res.RelevanceScore
		result = append(result, doc)
	}

	return result, nil
}
