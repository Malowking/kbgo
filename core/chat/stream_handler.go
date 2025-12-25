package chat

import (
	"context"
	"path/filepath"
	"time"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/logic/retriever"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// StreamHandler 流式聊天处理器
type StreamHandler struct{}

// NewStreamHandler 创建流式聊天处理器
func NewStreamHandler() *StreamHandler {
	return &StreamHandler{}
}

// StreamChat 处理流式聊天请求
func (h *StreamHandler) StreamChat(ctx context.Context, req *v1.ChatReq, uploadedFiles []*common.MultimodalFile) error {
	// 获取检索配置
	cfg := retriever.GetRetrieverConfig()

	// 1. 执行知识检索
	type retrievalResult struct {
		documents         []*schema.Document
		retrieverMetadata map[string]interface{}
		err               error
	}

	type mcpResult struct {
		mcpResults  []*v1.MCPResult
		mcpMetadata []map[string]interface{}
		err         error
	}

	type fileParseResult struct {
		multimodalFiles []*common.MultimodalFile // 多模态文件（图片、音频、视频等）
		fileContent     string                   // 文档文件的解析文本内容
		fileImages      []string                 // 文档文件中提取的图片路径
		err             error
	}

	retrievalChan := make(chan retrievalResult, 1)
	fileParseChan := make(chan fileParseResult, 1)

	// 并行执行检索
	go func() {
		var result retrievalResult
		if req.EnableRetriever && req.KnowledgeId != "" {
			// 确定使用的检索模式：优先使用请求中的参数，否则使用配置默认值
			retrieveMode := cfg.RetrieveMode
			if req.RetrieveMode != "" {
				retrieveMode = req.RetrieveMode
			}

			// chat接口默认开启查询重写，重写次数为3
			rewriteAttempts := 3

			retrieverRes, err := retriever.ProcessRetrieval(ctx, &v1.RetrieverReq{
				Question:         req.Question,
				EmbeddingModelID: req.EmbeddingModelID,
				RerankModelID:    req.RerankModelID,
				TopK:             req.TopK,
				Score:            req.Score,
				KnowledgeId:      req.KnowledgeId,
				EnableRewrite:    true,
				RewriteAttempts:  rewriteAttempts,
				RetrieveMode:     retrieveMode,
			})
			if err != nil {
				g.Log().Errorf(ctx, "知识检索失败: %v", err)
				result.err = err
			} else {
				result.documents = retrieverRes.Document
				result.retrieverMetadata = map[string]interface{}{
					"type":           "retriever",
					"knowledge_id":   req.KnowledgeId,
					"top_k":          req.TopK,
					"score":          req.Score,
					"document_count": len(retrieverRes.Document),
				}
				g.Log().Infof(ctx, "知识检索完成，返回 %d 个文档", len(retrieverRes.Document))
			}
		}
		retrievalChan <- result
	}()

	// 并行处理文件
	go func() {
		var result fileParseResult
		if len(uploadedFiles) > 0 {
			g.Log().Infof(ctx, "Stream handler - Processing %d uploaded files", len(uploadedFiles))

			// 分离多模态文件和文档文件
			var multimodalFiles []*common.MultimodalFile
			var documentFiles []*common.MultimodalFile

			for _, file := range uploadedFiles {
				if file.FileType == common.FileTypeImage ||
					file.FileType == common.FileTypeAudio ||
					file.FileType == common.FileTypeVideo {
					multimodalFiles = append(multimodalFiles, file)
				} else {
					documentFiles = append(documentFiles, file)
				}
			}

			g.Log().Infof(ctx, "Stream handler - Separated into %d multimodal files and %d document files",
				len(multimodalFiles), len(documentFiles))

			result.multimodalFiles = multimodalFiles

			// 如果有文档文件，调用Python服务解析
			if len(documentFiles) > 0 {
				g.Log().Infof(ctx, "Stream handler - Parsing %d document files", len(documentFiles))
				fileContent, fileImages, err := chat.ParseDocumentFiles(ctx, documentFiles)
				if err != nil {
					g.Log().Errorf(ctx, "Stream handler - Failed to parse document files: %v", err)
					result.err = err
				} else {
					result.fileContent = fileContent
					result.fileImages = fileImages
					g.Log().Infof(ctx, "Stream handler - Parsed documents: %d chars of text, %d images",
						len(fileContent), len(fileImages))
				}
			}
		}
		fileParseChan <- result
	}()

	// 等待并行任务完成
	retrievalRes := <-retrievalChan
	fileParseRes := <-fileParseChan

	if retrievalRes.err != nil {
		return retrievalRes.err
	}

	if fileParseRes.err != nil {
		g.Log().Warningf(ctx, "File parsing failed: %v, continuing without file content", fileParseRes.err)
	}

	// 获取检索文档
	var documents []*schema.Document
	documents = retrievalRes.documents

	// 如果有解析的文档内容，添加到 documents 中
	if fileParseRes.fileContent != "" {
		documents = append(documents, &schema.Document{
			ID:       "uploaded_document",
			Content:  fileParseRes.fileContent,
			MetaData: map[string]interface{}{"source": "user_upload", "type": "document"},
		})
		g.Log().Infof(ctx, "Added parsed document content to documents (%d chars)", len(fileParseRes.fileContent))
	}

	// 2. 执行MCP工具调用
	var mcpRes mcpResult
	if req.UseMCP {
		g.Log().Infof(ctx, "开始执行MCP工具调用...")
		mcpHandler := NewMCPHandler()
		// 传入检索到的文档和文件解析内容
		_, mcpResults, mcpFinalAnswer, err := mcpHandler.CallMCPToolsWithLLM(ctx, req, documents, fileParseRes.fileContent)
		if err != nil {
			g.Log().Errorf(ctx, "MCP智能工具调用失败: %v", err)
			mcpRes.err = err
		} else {
			g.Log().Infof(ctx, "MCP工具调用完成，返回 %d 个结果", len(mcpResults))
			mcpRes.mcpResults = mcpResults
			mcpRes.mcpMetadata = make([]map[string]interface{}, len(mcpResults))
			for i, res := range mcpResults {
				mcpRes.mcpMetadata[i] = map[string]interface{}{
					"type":         "mcp",
					"service_name": res.ServiceName,
					"tool_name":    res.ToolName,
					"content":      res.Content,
				}
			}
			if mcpFinalAnswer != "" {
				g.Log().Infof(ctx, "MCP returned final answer (length: %d)", len(mcpFinalAnswer))
			}
		}
	}

	// 获取Chat实例
	chatI := chat.GetChat()

	// 使用文件解析结果中的多模态文件
	multimodalFiles := fileParseRes.multimodalFiles

	// 如果有从文档中提取的图片，将它们转换为 MultimodalFile
	if len(fileParseRes.fileImages) > 0 {
		g.Log().Infof(ctx, "Adding %d extracted images from documents", len(fileParseRes.fileImages))
		for _, imagePath := range fileParseRes.fileImages {
			multimodalFiles = append(multimodalFiles, &common.MultimodalFile{
				FileName:     filepath.Base(imagePath),
				FilePath:     imagePath,
				RelativePath: imagePath,
				FileType:     common.FileTypeImage,
			})
		}
	}

	// 记录开始时间
	start := time.Now()

	// 获取流式响应
	var streamReader schema.StreamReaderInterface[*schema.Message]
	var err error
	if len(multimodalFiles) > 0 {
		g.Log().Infof(ctx, "Using multimodal stream chat with %d files", len(multimodalFiles))
		streamReader, err = chatI.GetAnswerStreamWithFiles(ctx, req.ModelID, req.ConvID, documents, req.Question, multimodalFiles, req.JsonFormat)
	} else {
		streamReader, err = chatI.GetAnswerStream(ctx, req.ModelID, req.ConvID, documents, req.Question, req.SystemPrompt, req.JsonFormat)
	}
	if err != nil {
		g.Log().Error(ctx, err)
		return err
	}
	defer streamReader.Close()

	// 在流式响应中添加MCP结果
	allDocuments := h.buildAllDocuments(documents, mcpRes.mcpResults)

	// 准备元数据
	metadata := h.buildMetadata(retrievalRes.retrieverMetadata, mcpRes.mcpMetadata)

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

// buildAllDocuments 构建所有文档
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
func (h *StreamHandler) handleStreamResponse(ctx context.Context, streamReader schema.StreamReaderInterface[*schema.Message], allDocuments []*schema.Document, start time.Time, convID string, metadata map[string]interface{}, chatI interface{}) error {
	// 直接发送流式响应到客户端
	// 注意：完整消息已经在 chat.go 的 goroutine 中保存，这里不需要重复保存
	err := common.SteamResponse(ctx, streamReader, allDocuments)
	if err != nil {
		return err
	}

	return nil
}
