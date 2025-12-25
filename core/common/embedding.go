package common

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/Malowking/kbgo/core/errors"
)

// EmbeddingConfig 接口，用于提取embedding配置
type EmbeddingConfig interface {
	GetAPIKey() string
	GetBaseURL() string
	GetEmbeddingModel() string
}

// CustomEmbedder 自定义embedding客户端
type CustomEmbedder struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// EmbeddingRequest OpenAI embedding API请求结构
type EmbeddingRequest struct {
	Input      []string `json:"input"`
	Model      string   `json:"model"`
	Dimensions *int     `json:"dimensions,omitempty"`
}

// EmbeddingResponse OpenAI embedding API响应结构
type EmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
		Object    string    `json:"object"`
	} `json:"data"`
	Model  string `json:"model"`
	Object string `json:"object"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// ErrorResponse API错误响应
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

func NewEmbedding(ctx context.Context, conf EmbeddingConfig) (*CustomEmbedder, error) {
	apiKey := conf.GetAPIKey()
	baseURL := conf.GetBaseURL()
	model := conf.GetEmbeddingModel()

	if apiKey == "" {
		return nil, errors.Newf(errors.ErrInvalidParameter, "embedding apiKey is required")
	}
	if baseURL == "" {
		return nil, errors.Newf(errors.ErrInvalidParameter, "embedding baseURL is required")
	}
	if model == "" {
		return nil, errors.Newf(errors.ErrInvalidParameter, "embedding model not found")
	}

	// 创建自定义HTTP客户端，设置合理的超时时间
	httpClient := &http.Client{
		Timeout: 5 * time.Minute, // 总体超时5分钟
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second, // 连接超时
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   30 * time.Second, // TLS握手超时
			ResponseHeaderTimeout: 2 * time.Minute,  // 等待响应头超时
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
		},
	}

	return &CustomEmbedder{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		httpClient: httpClient,
	}, nil
}

// EmbedStrings 实现字符串数组的向量化
func (e *CustomEmbedder) EmbedStrings(ctx context.Context, texts []string, dimensions int) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	req := EmbeddingRequest{
		Input:      texts,
		Model:      e.model,
		Dimensions: &dimensions,
	}

	// 序列化请求
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "failed to marshal request: %v", err)
	}

	// 创建HTTP请求
	url := e.baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "failed to create request: %v", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.apiKey)

	// 发送请求
	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, errors.Newf(errors.ErrEmbeddingFailed, "HTTP %d: failed to decode error response: %v", resp.StatusCode, err)
		}
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "API error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
	}

	// 解析响应
	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "failed to decode response: %v", err)
	}

	// 验证响应数据
	if len(embResp.Data) != len(texts) {
		return nil, errors.Newf(errors.ErrEmbeddingFailed, "response data length (%d) doesn't match input length (%d)", len(embResp.Data), len(texts))
	}

	// 提取embedding向量并转换为float32
	result := make([][]float32, len(texts))
	for _, data := range embResp.Data {
		if data.Index >= len(result) {
			return nil, errors.Newf(errors.ErrEmbeddingFailed, "invalid embedding index: %d", data.Index)
		}
		// 将float64向量转换为float32
		float32Vec := make([]float32, len(data.Embedding))
		for i, v := range data.Embedding {
			float32Vec[i] = float32(v)
		}
		result[data.Index] = float32Vec
	}

	return result, nil
}
