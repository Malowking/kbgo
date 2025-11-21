package chat

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// StreamHandler 流式聊天处理器
type StreamHandler struct{}

// NewStreamHandler 创建流式聊天处理器
func NewStreamHandler() *StreamHandler {
	return &StreamHandler{}
}

// ProcessStreamChat 处理流式聊天请求
func (h *StreamHandler) StreamChat(ctx context.Context, req *v1.ChatReq) error {
	var streamReader *schema.StreamReader[*schema.Message]

	// 获取检索配置
	cfg := rag.GetRetrieverConfig()

	// 如果启用了知识库检索且提供了知识库ID，则进行检索
	var documents []*schema.Document
	var retrieverMetadata map[string]interface{}
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
			g.Log().Error(ctx, err)
			return err
		}
		documents = retriever.Document

		// 准备检索器元数据
		retrieverMetadata = map[string]interface{}{
			"type":           "retriever",
			"knowledge_id":   req.KnowledgeId,
			"top_k":          req.TopK,
			"score":          req.Score,
			"document_count": len(retriever.Document),
		}
	}

	var mcpResults []*v1.MCPResult
	var mcpMetadata []map[string]interface{}
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

			// 准备MCP元数据
			mcpMetadata = make([]map[string]interface{}, len(mcpRes))
			for i, result := range mcpRes {
				mcpMetadata[i] = map[string]interface{}{
					"type":         "mcp",
					"service_name": result.ServiceName,
					"tool_name":    result.ToolName,
					"content":      result.Content,
				}
			}
		}
	}

	// 获取Chat实例
	chatI := chat.GetChat()

	// 记录开始时间
	start := time.Now()

	// 获取流式响应
	streamReader, err := chatI.GetAnswerStream(ctx, req.ConvID, documents, req.Question)
	if err != nil {
		g.Log().Error(ctx, err)
		return err
	}
	defer streamReader.Close()

	// 在流式响应中添加MCP结果
	allDocuments := h.buildAllDocuments(documents, mcpResults)

	// 准备元数据
	metadata := h.buildMetadata(retrieverMetadata, mcpMetadata)

	// 将元数据添加到所有文档中
	if len(metadata) > 0 {
		for _, doc := range allDocuments {
			if doc.MetaData == nil {
				doc.MetaData = make(map[string]interface{})
			}
			doc.MetaData["chat_metadata"] = metadata
		}
	}

	// 处理流式响应和内容收集
	err = h.handleStreamResponse(ctx, streamReader, allDocuments, start, req.ConvID, metadata, chatI)
	if err != nil {
		g.Log().Error(ctx, err)
		return err
	}

	return nil
}

// buildAllDocuments 构建所有文档（包括MCP结果）
func (h *StreamHandler) buildAllDocuments(documents []*schema.Document, mcpResults []*v1.MCPResult) []*schema.Document {
	var allDocuments []*schema.Document
	allDocuments = append(allDocuments, documents...)

	// 添加MCP结果作为文档
	for _, mcpResult := range mcpResults {
		mcpDoc := &schema.Document{
			ID:      "mcp_" + mcpResult.ServiceName + "_" + mcpResult.ToolName,
			Content: mcpResult.Content,
			MetaData: map[string]interface{}{
				"source":       "mcp",
				"service_name": mcpResult.ServiceName,
				"tool_name":    mcpResult.ToolName,
			},
		}
		allDocuments = append(allDocuments, mcpDoc)
	}

	return allDocuments
}

// buildMetadata 构建元数据
func (h *StreamHandler) buildMetadata(retrieverMetadata map[string]interface{}, mcpMetadata []map[string]interface{}) map[string]interface{} {
	metadata := map[string]interface{}{}
	if retrieverMetadata != nil {
		metadata["retriever"] = retrieverMetadata
	}
	if mcpMetadata != nil {
		metadata["mcp_tools"] = mcpMetadata
	}
	return metadata
}

// handleStreamResponse 处理流式响应
func (h *StreamHandler) handleStreamResponse(ctx context.Context, streamReader *schema.StreamReader[*schema.Message], allDocuments []*schema.Document, start time.Time, convID string, metadata map[string]interface{}, chatI interface{}) error {
	// 收集流式响应内容以保存完整消息
	var fullContent strings.Builder

	// 创建两个管道用于复制流
	srs := streamReader.Copy(2)
	streamReader = srs[0]     // 用于原始响应
	collectorReader := srs[1] // 用于收集内容

	// 启动一个 goroutine 来收集内容
	go func() {
		defer collectorReader.Close()
		for {
			msg, err := collectorReader.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				g.Log().Errorf(ctx, "Error collecting stream content: %v", err)
				break
			}
			if msg != nil {
				fullContent.WriteString(msg.Content)
			}
		}

		// 计算延迟
		_ = time.Since(start).Milliseconds()

		// TODO: 这里可能需要将latencyMs和tokens_used传递给前端或者其他地方

		// 流式响应结束后，保存带元数据的完整消息
		if len(metadata) > 0 {
			fullMessage := fullContent.String()
			// 注意：这里需要类型断言或重新设计接口
			if chatInstance, ok := chatI.(interface {
				SaveStreamingMessageWithMetadata(string, string, map[string]interface{})
			}); ok {
				chatInstance.SaveStreamingMessageWithMetadata(convID, fullMessage, metadata)
			}
		}
	}()

	err := common.SteamResponse(ctx, streamReader, allDocuments)
	if err != nil {
		return err
	}

	return nil
}
