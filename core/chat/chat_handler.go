package chat

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ChatHandler Chat handler
type ChatHandler struct{}

// NewChatHandler Create chat handler
func NewChatHandler() *ChatHandler {
	return &ChatHandler{}
}

// Handle basic chat request (non-streaming)
func (h *ChatHandler) Chat(ctx context.Context, req *v1.ChatReq) (*v1.ChatRes, error) {
	// Get retriever configuration
	cfg := rag.GetRetrieverConfig()

	// Initialize response
	res := &v1.ChatRes{}

	// If knowledge base retrieval is enabled and knowledge base ID is provided, perform retrieval
	var documents []*schema.Document
	if req.EnableRetriever && req.KnowledgeId != "" {
		retrieverHandler := NewRetrieverHandler()
		retriever, err := retrieverHandler.ProcessRetrieval(ctx, &v1.RetrieverReq{
			Question:        req.Question,
			TopK:            req.TopK,
			Score:           req.Score,
			KnowledgeId:     req.KnowledgeId,
			EnableRewrite:   cfg.EnableRewrite,
			RewriteAttempts: cfg.RewriteAttempts,
			RetrieveMode:    cfg.RetrieveMode,
		})
		if err != nil {
			return nil, err
		}
		documents = retriever.Document
		res.References = retriever.Document
	}

	var mcpResults []*v1.MCPResult
	// 如果启用MCP，则执行MCP逻辑
	if req.UseMCP {
		// 使用新的智能工具调用逻辑
		mcpHandler := NewMCPHandler()
		mcpDocs, mcpRes, err := mcpHandler.CallMCPToolsWithLLM(ctx, req)
		if err != nil {
			g.Log().Errorf(ctx, "MCP intelligent tool call failed: %v", err)
		} else {
			// 将MCP结果合并到上下文中
			documents = append(documents, mcpDocs...)
			mcpResults = mcpRes
		}
	}

	// Get Chat instance and generate answer
	chatI := chat.GetChat()

	answer, err := chatI.GetAnswer(ctx, req.ConvID, documents, req.Question)
	if err != nil {
		return nil, err
	}

	res.Answer = answer
	if len(mcpResults) > 0 {
		res.MCPResults = mcpResults
	}

	// Note: GetAnswer method has already saved the assistant message, no need to save again

	return res, nil
}
