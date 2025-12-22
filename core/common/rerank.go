package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/Malowking/kbgo/core/errors"
	"golang.org/x/sync/errgroup"
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

// AggregateStrategy 定义聚合策略类型
type AggregateStrategy string

const (
	// AggregateStrategyMax Max Pooling策略：取所有sub-chunk分数的最大值
	AggregateStrategyMax AggregateStrategy = "max"
	// AggregateStrategyTopKMean Top-K Mean策略：取前K个sub-chunk分数的平均值
	AggregateStrategyTopKMean AggregateStrategy = "topk_mean"
	// AggregateStrategyMean Mean策略：取所有sub-chunk分数的平均值
	AggregateStrategyMean AggregateStrategy = "mean"
)

// SubChunkConfig 子切片配置
type SubChunkConfig struct {
	// SubChunkSize 每个子切片的字符大小（默认 250）
	SubChunkSize int
	// OverlapSize 子切片之间的重叠字符数（默认 50）
	OverlapSize int
	// AggregateStrategy 聚合策略（默认 "max"）
	AggregateStrategy AggregateStrategy
	// TopKForMean Top-K Mean策略中的K值（默认 2）
	TopKForMean int
	// MaxSubChunksPerDoc 每个文档最多切分的子片段数（默认 0 表示不限制）
	MaxSubChunksPerDoc int
	// ScoreThreshold 相对阈值：过滤低分子切片，保留 score >= max_score * threshold 的子切片（默认 0.6）
	// 设置为 0 表示不过滤
	ScoreThreshold float64
}

// DefaultSubChunkConfig 返回默认的子切片配置
func DefaultSubChunkConfig() SubChunkConfig {
	return SubChunkConfig{
		SubChunkSize:       250,
		OverlapSize:        50,
		AggregateStrategy:  AggregateStrategyMax,
		TopKForMean:        2,
		MaxSubChunksPerDoc: 0,   // 不限制
		ScoreThreshold:     0.6, // 相对阈值 60%
	}
}

// subChunkResult 子切片的 rerank 结果
type subChunkResult struct {
	docID      string
	subChunkID int
	score      float64
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
			return nil, errors.New(errors.ErrInvalidParameter, "rerank baseURL is required")
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
		return nil, errors.Newf(errors.ErrRerankFailed, "failed to marshal request: %v", err)
	}

	// 创建HTTP请求
	url := r.baseURL + "/rerank"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Newf(errors.ErrRerankFailed, "failed to create request: %v", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)

	// 发送请求
	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Newf(errors.ErrRerankFailed, "failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		var errResp RerankErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, errors.Newf(errors.ErrRerankFailed, "HTTP %d: failed to decode error response: %v", resp.StatusCode, err)
		}
		return nil, errors.Newf(errors.ErrRerankFailed, "API error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
	}

	// 解析响应
	var rerankResp RerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&rerankResp); err != nil {
		return nil, errors.Newf(errors.ErrRerankFailed, "failed to decode response: %v", err)
	}

	// 验证响应数据
	if len(rerankResp.Results) == 0 {
		return []RerankDocument{}, nil
	}

	// 构造返回结果
	result := make([]RerankDocument, 0, len(rerankResp.Results))
	for _, res := range rerankResp.Results {
		if res.Index >= len(docs) {
			return nil, errors.Newf(errors.ErrRerankFailed, "invalid result index: %d", res.Index)
		}
		doc := docs[res.Index]
		doc.Score = res.RelevanceScore
		result = append(result, doc)
	}

	return result, nil
}

// SplitIntoSubChunks 将文档内容切分为多个子切片（支持滑窗重叠）
// content: 要切分的文档内容
// subChunkSize: 每个子切片的字符大小
// overlapSize: 子切片之间的重叠字符数
// maxSubChunks: 最多切分的子片段数（0 表示不限制）
func SplitIntoSubChunks(content string, subChunkSize, overlapSize, maxSubChunks int) []string {
	if content == "" {
		return []string{}
	}

	// 如果内容长度小于等于 subChunkSize，直接返回整个内容
	contentLen := len([]rune(content)) // 使用 rune 计数以正确处理多字节字符
	if contentLen <= subChunkSize {
		return []string{content}
	}

	// 转换为 rune 数组以正确处理多字节字符（如中文）
	runes := []rune(content)
	var subChunks []string

	// 计算步长（subChunkSize - overlapSize）
	step := subChunkSize - overlapSize
	if step <= 0 {
		step = subChunkSize // 如果重叠大于等于 chunk size，则不重叠
	}

	// 滑窗切分
	for start := 0; start < contentLen; start += step {
		end := start + subChunkSize
		if end > contentLen {
			end = contentLen
		}

		subChunk := string(runes[start:end])
		subChunks = append(subChunks, subChunk)

		// 如果已经到达末尾，退出循环
		if end >= contentLen {
			break
		}

		// 如果设置了最大子片段数限制，检查是否已达到限制
		if maxSubChunks > 0 && len(subChunks) >= maxSubChunks {
			break
		}
	}

	return subChunks
}

// aggregateScores 根据策略聚合多个子切片的分数
func aggregateScores(scores []float64, strategy AggregateStrategy, topK int) float64 {
	if len(scores) == 0 {
		return 0.0
	}

	switch strategy {
	case AggregateStrategyMax:
		// Max Pooling：返回最大分数
		return maxScore(scores)

	case AggregateStrategyTopKMean:
		// Top-K Mean：返回前K个分数的平均值
		if topK <= 0 {
			topK = 2 // 默认取前2个
		}
		return topKMeanScore(scores, topK)

	case AggregateStrategyMean:
		// Mean：返回所有分数的平均值
		return meanScore(scores)

	default:
		// 默认使用 Max Pooling
		return maxScore(scores)
	}
}

// maxScore 返回分数数组的最大值
func maxScore(scores []float64) float64 {
	if len(scores) == 0 {
		return 0.0
	}

	maxVal := scores[0]
	for _, score := range scores[1:] {
		if score > maxVal {
			maxVal = score
		}
	}
	return maxVal
}

// topKMeanScore 返回前K个最高分数的平均值
func topKMeanScore(scores []float64, k int) float64 {
	if len(scores) == 0 {
		return 0.0
	}

	// 复制分数数组并排序（从大到小）
	sortedScores := make([]float64, len(scores))
	copy(sortedScores, scores)
	sort.Slice(sortedScores, func(i, j int) bool {
		return sortedScores[i] > sortedScores[j]
	})

	// 取前K个
	if k > len(sortedScores) {
		k = len(sortedScores)
	}

	// 计算平均值
	sum := 0.0
	for i := 0; i < k; i++ {
		sum += sortedScores[i]
	}

	return sum / float64(k)
}

// meanScore 返回所有分数的平均值
func meanScore(scores []float64) float64 {
	if len(scores) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, score := range scores {
		sum += score
	}

	return sum / float64(len(scores))
}

// filterLowScoreSubChunks 使用相对阈值过滤低分子切片
// 策略：保留 score >= max_score * threshold 的子切片
// 优点：自动适配不同 query，避免某些 query 整体分数偏低的问题
func filterLowScoreSubChunks(scores []float64, threshold float64) []float64 {
	if len(scores) == 0 || threshold <= 0 {
		return scores
	}

	// 找到最大分数
	maxScoreVal := maxScore(scores)
	if maxScoreVal == 0 {
		return scores
	}

	// 计算相对阈值
	relativeThreshold := maxScoreVal * threshold

	// 过滤低分子切片
	var filtered []float64
	for _, score := range scores {
		if score >= relativeThreshold {
			filtered = append(filtered, score)
		}
	}

	// 如果所有分数都被过滤掉了，保留最高分
	if len(filtered) == 0 && len(scores) > 0 {
		filtered = append(filtered, maxScoreVal)
	}

	return filtered
}

// RerankWithSubChunks 使用子切片滑窗并行 rerank（核心优化函数）
// 对每个 chunk 进行子切分，然后并行调用 rerank API，最后按策略聚合分数
func (r *CustomReranker) RerankWithSubChunks(ctx context.Context, query string, docs []RerankDocument, topK int, config SubChunkConfig) ([]RerankDocument, error) {
	if len(docs) == 0 {
		return []RerankDocument{}, nil
	}

	startTime := time.Now()

	// 验证配置
	if config.SubChunkSize <= 0 {
		config = DefaultSubChunkConfig()
	}

	// 第一步：为每个文档生成子切片
	type docWithSubChunks struct {
		docID     string
		subChunks []string
	}

	var allSubChunks []docWithSubChunks
	totalSubChunks := 0

	for _, doc := range docs {
		subChunks := SplitIntoSubChunks(doc.Content, config.SubChunkSize, config.OverlapSize, config.MaxSubChunksPerDoc)
		allSubChunks = append(allSubChunks, docWithSubChunks{
			docID:     doc.ID,
			subChunks: subChunks,
		})
		totalSubChunks += len(subChunks)
	}

	// 第二步：并行调用 rerank API（关键优化点）
	// 使用 errgroup 进行并发控制和错误处理
	g, gCtx := errgroup.WithContext(ctx)

	// 使用 channel 收集所有子切片的 rerank 结果
	resultChan := make(chan subChunkResult, totalSubChunks)

	// 构建全局子切片索引映射
	subChunkToIndex := make(map[string]int) // 子切片内容到全局索引的映射
	allSubChunkContents := []string{}       // 所有子切片内容的有序列表

	for _, dwc := range allSubChunks {
		for _, subChunk := range dwc.subChunks {
			allSubChunkContents = append(allSubChunkContents, subChunk)
			subChunkToIndex[subChunk] = len(allSubChunkContents) - 1
		}
	}

	// 并发批量 rerank 策略：
	// 由于 rerank API 支持一次调用多个文档，我们可以分批并行调用
	// 这里使用一个合理的批大小（如 20-50 个 sub-chunk 一批）
	batchSize := 30
	numBatches := int(math.Ceil(float64(totalSubChunks) / float64(batchSize)))

	for batchIdx := 0; batchIdx < numBatches; batchIdx++ {
		batchIdx := batchIdx // 捕获循环变量
		g.Go(func() error {
			start := batchIdx * batchSize
			end := (batchIdx + 1) * batchSize
			if end > len(allSubChunkContents) {
				end = len(allSubChunkContents)
			}

			batchContents := allSubChunkContents[start:end]

			// 为这批子切片构造临时文档
			batchDocs := make([]RerankDocument, len(batchContents))
			for i, content := range batchContents {
				batchDocs[i] = RerankDocument{
					ID:      fmt.Sprintf("subchunk_%d", start+i),
					Content: content,
					Score:   0,
				}
			}

			// 调用原有的 Rerank 方法
			// topN 设置为 len(batchDocs) 以获取所有结果的分数
			results, err := r.Rerank(gCtx, query, batchDocs, len(batchDocs))
			if err != nil {
				return errors.Newf(errors.ErrRerankFailed, "batch %d rerank failed: %v", batchIdx, err)
			}

			// 将结果发送到 channel
			// 注意：需要将批次内的索引映射回文档ID
			for _, result := range results {
				// result.ID 是 subchunk_X 格式，需要映射回原始文档
				// 通过 content 查找原始文档ID
				content := result.Content
				if content == "" {
					// 如果 API 没有返回 content，使用原始 batchDocs 中的 content
					for _, batchDoc := range batchDocs {
						if batchDoc.ID == result.ID {
							content = batchDoc.Content
							break
						}
					}
				}

				// 查找这个 sub-chunk 属于哪个文档
				found := false
				for _, dwc := range allSubChunks {
					for subChunkIdx, subChunk := range dwc.subChunks {
						if subChunk == content {
							resultChan <- subChunkResult{
								docID:      dwc.docID,
								subChunkID: subChunkIdx,
								score:      result.Score,
							}
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}

			return nil
		})
	}

	// 等待所有并行任务完成
	if err := g.Wait(); err != nil {
		return nil, errors.Newf(errors.ErrRerankFailed, "parallel rerank failed: %v", err)
	}
	close(resultChan)

	// 第三步：聚合每个文档的子切片分数
	docScoresMap := make(map[string][]float64) // docID -> sub-chunk scores

	for result := range resultChan {
		docScoresMap[result.docID] = append(docScoresMap[result.docID], result.score)
	}

	// 第四步：计算每个文档的最终分数并排序
	var finalResults []RerankDocument

	for _, doc := range docs {
		scores, exists := docScoresMap[doc.ID]
		if !exists || len(scores) == 0 {
			// 如果没有找到分数，给一个默认的低分
			doc.Score = 0.0
		} else {
			// ✅ 优化：使用相对阈值过滤低分子切片
			// 保留 score >= max_score * threshold 的子切片
			// 自动适配不同 query，避免某些 query 整体分数偏低的问题
			filteredScores := filterLowScoreSubChunks(scores, config.ScoreThreshold)

			// 根据配置的聚合策略计算最终分数（基于过滤后的分数）
			doc.Score = aggregateScores(filteredScores, config.AggregateStrategy, config.TopKForMean)
		}
		finalResults = append(finalResults, doc)
	}

	// 按分数降序排序
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].Score > finalResults[j].Score
	})

	// 截取 TopK
	if topK > 0 && topK < len(finalResults) {
		finalResults = finalResults[:topK]
	}

	// 性能日志
	elapsed := time.Since(startTime)
	_ = elapsed // 可以用于日志记录

	return finalResults, nil
}
