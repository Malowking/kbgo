package chat

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ChatHandler 聊天处理器
type ChatHandler struct{}

// NewChatHandler 创建聊天处理器
func NewChatHandler() *ChatHandler {
	return &ChatHandler{}
}

// 处理基础聊天请求（非流式）
func (h *ChatHandler) Chat(ctx context.Context, req *v1.ChatReq) (*v1.ChatRes, error) {
	// 获取检索配置
	cfg := rag.GetRetrieverConfig()

	// 初始化返回结果
	res := &v1.ChatRes{}

	// 如果启用了知识库检索且提供了知识库ID，则进行检索
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
			g.Log().Errorf(ctx, "MCP智能工具调用失败: %v", err)
		} else {
			// 将MCP结果合并到上下文中
			documents = append(documents, mcpDocs...)
			mcpResults = mcpRes
		}
	}

	// 获取Chat实例并生成答案
	chatI := chat.GetChat()

	answer, err := chatI.GetAnswer(ctx, req.ConvID, documents, req.Question)
	if err != nil {
		return nil, err
	}

	res.Answer = answer
	if len(mcpResults) > 0 {
		res.MCPResults = mcpResults
	}

	// 注意：GetAnswer方法已经保存了助手消息，这里不需要再保存

	return res, nil
}
