package chat

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/logic/retriever"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ChatHandler Chat handler
type ChatHandler struct{}

// NewChatHandler Create chat handler
func NewChatHandler() *ChatHandler {
	return &ChatHandler{}
}

// Handle basic chat request (non-streaming)
func (h *ChatHandler) Chat(ctx context.Context, req *v1.ChatReq, uploadedFiles []*common.MultimodalFile) (*v1.ChatRes, error) {
	// Get retriever configuration
	cfg := retriever.GetRetrieverConfig()

	// Initialize response
	res := &v1.ChatRes{}

	// 使用channel并行处理检索和MCP
	type retrievalResult struct {
		documents []*schema.Document
		err       error
	}

	type mcpResult struct {
		documents  []*schema.Document
		mcpResults []*v1.MCPResult
		err        error
	}

	retrievalChan := make(chan retrievalResult, 1)
	mcpChan := make(chan mcpResult, 1)

	// 并行执行检索
	go func() {
		var result retrievalResult
		if req.EnableRetriever && req.KnowledgeId != "" {
			g.Log().Infof(ctx, "Chat handler - Triggering retrieval with TopK: %d, Score: %f", req.TopK, req.Score)

			// 确定使用的检索模式：优先使用请求中的参数，否则使用配置默认值
			retrieveMode := cfg.RetrieveMode
			if req.RetrieveMode != "" {
				retrieveMode = req.RetrieveMode
			}

			// chat接口默认开启查询重写，重写次数为3
			enableRewrite := true
			rewriteAttempts := 3

			retrieverRes, err := retriever.ProcessRetrieval(ctx, &v1.RetrieverReq{
				Question:         req.Question,
				EmbeddingModelID: req.EmbeddingModelID,
				RerankModelID:    req.RerankModelID,
				TopK:             req.TopK,
				Score:            req.Score,
				KnowledgeId:      req.KnowledgeId,
				EnableRewrite:    enableRewrite,
				RewriteAttempts:  rewriteAttempts,
				RetrieveMode:     retrieveMode,
			})
			if err != nil {
				result.err = err
			} else {
				result.documents = retrieverRes.Document
				g.Log().Infof(ctx, "Chat handler - Retrieved %d documents", len(retrieverRes.Document))
			}
		} else {
			if !req.EnableRetriever {
				g.Log().Infof(ctx, "Chat handler - Retrieval disabled by EnableRetriever=false")
			}
			if req.KnowledgeId == "" {
				g.Log().Infof(ctx, "Chat handler - Retrieval skipped due to empty KnowledgeId")
			}
		}
		retrievalChan <- result
	}()

	// 并行执行MCP
	go func() {
		var result mcpResult
		if req.UseMCP {
			mcpHandler := NewMCPHandler()
			mcpDocs, mcpRes, err := mcpHandler.CallMCPToolsWithLLM(ctx, req)
			if err != nil {
				g.Log().Errorf(ctx, "MCP intelligent tool call failed: %v", err)
				result.err = err
			} else {
				result.documents = mcpDocs
				result.mcpResults = mcpRes
			}
		}
		mcpChan <- result
	}()

	// 等待并行任务完成
	retrievalRes := <-retrievalChan
	mcpRes := <-mcpChan

	// 处理检索结果
	var documents []*schema.Document
	if retrievalRes.err != nil {
		return nil, retrievalRes.err
	}
	documents = retrievalRes.documents
	res.References = retrievalRes.documents

	// 合并MCP结果
	if mcpRes.err == nil && len(mcpRes.documents) > 0 {
		documents = append(documents, mcpRes.documents...)
		res.MCPResults = mcpRes.mcpResults
	}

	// Get Chat instance and generate answer
	chatI := chat.GetChat()

	// 过滤出多模态文件（只有图片、音频、视频才使用多模态）
	var multimodalFiles []*common.MultimodalFile
	for _, file := range uploadedFiles {
		if file.FileType == common.FileTypeImage ||
			file.FileType == common.FileTypeAudio ||
			file.FileType == common.FileTypeVideo {
			multimodalFiles = append(multimodalFiles, file)
		} else {
			g.Log().Infof(ctx, "Skipping non-multimodal file: %s (type: %s)", file.FileName, file.FileType)
		}
	}

	// 如果有多模态文件（图片/音频/视频），则使用多模态消息生成答案
	var answer string
	var err error
	if len(multimodalFiles) > 0 {
		g.Log().Infof(ctx, "Using multimodal chat with %d files", len(multimodalFiles))
		answer, err = chatI.GetAnswerWithFiles(ctx, req.ModelID, req.ConvID, documents, req.Question, multimodalFiles)
	} else {
		answer, err = chatI.GetAnswer(ctx, req.ModelID, req.ConvID, documents, req.Question)
	}
	if err != nil {
		return nil, err
	}

	res.Answer = answer

	// Note: GetAnswer method has already saved the assistant message, no need to save again

	return res, nil
}
