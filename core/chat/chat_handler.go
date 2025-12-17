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
	// 加载Agent预设配置（如果会话关联了Agent预设）
	req = h.loadAgentPresetConfig(ctx, req)

	// Get retriever configuration
	cfg := retriever.GetRetrieverConfig()

	// Initialize response
	res := &v1.ChatRes{}

	// 定义并行任务的结果类型
	type retrievalResult struct {
		documents []*schema.Document
		err       error
	}

	type fileParseResult struct {
		multimodalFiles []*common.MultimodalFile // 多模态文件（图片、音频、视频等）
		fileContent     string                   // 文档文件的解析文本内容
		fileImages      []string                 // 文档文件中提取的图片路径
		err             error
	}

	// 创建channels用于并行任务
	retrievalChan := make(chan retrievalResult, 1)
	fileParseChan := make(chan fileParseResult, 1)

	// 1. 并行执行知识检索
	go func() {
		var result retrievalResult
		if req.EnableRetriever && req.KnowledgeId != "" {
			g.Log().Infof(ctx, "Chat handler - Triggering retrieval with TopK: %d, Score: %f", req.TopK, req.Score)

			// 确定使用的检索模式
			retrieveMode := cfg.RetrieveMode
			if req.RetrieveMode != "" {
				retrieveMode = req.RetrieveMode
			}

			// chat接口默认开启查询重写
			rewriteAttempts := 3

			retrieverRes, err := retriever.ProcessRetrieval(ctx, &v1.RetrieverReq{
				Question:         req.Question,
				EmbeddingModelID: req.EmbeddingModelID,
				RerankModelID:    req.RerankModelID,
				TopK:             req.TopK,
				Score:            req.Score,
				KnowledgeId:      req.KnowledgeId,
				EnableRewrite:    true, // chat接口默认开启查询重写
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
				g.Log().Infof(ctx, "Chat handler - Retrieval disabled")
			}
			if req.KnowledgeId == "" {
				g.Log().Infof(ctx, "Chat handler - No knowledge base specified")
			}
		}
		retrievalChan <- result
	}()

	// 2. 并行处理文件（如果有上传文件）
	go func() {
		var result fileParseResult
		if len(uploadedFiles) > 0 {
			g.Log().Infof(ctx, "Chat handler - Processing %d uploaded files", len(uploadedFiles))

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

			g.Log().Infof(ctx, "Chat handler - Separated into %d multimodal files and %d document files",
				len(multimodalFiles), len(documentFiles))

			result.multimodalFiles = multimodalFiles

			// 如果有文档文件，调用Python服务解析
			if len(documentFiles) > 0 {
				g.Log().Infof(ctx, "Chat handler - Parsing %d document files", len(documentFiles))
				fileContent, fileImages, err := chat.ParseDocumentFiles(ctx, documentFiles)
				if err != nil {
					g.Log().Errorf(ctx, "Chat handler - Failed to parse document files: %v", err)
					result.err = err
				} else {
					result.fileContent = fileContent
					result.fileImages = fileImages
					g.Log().Infof(ctx, "Chat handler - Parsed documents: %d chars of text, %d images",
						len(fileContent), len(fileImages))
				}
			}
		}
		fileParseChan <- result
	}()

	// 3. 等待并行任务完成
	retrievalRes := <-retrievalChan
	fileParseRes := <-fileParseChan

	// 处理检索错误
	if retrievalRes.err != nil {
		return nil, retrievalRes.err
	}

	// 处理文件解析错误
	if fileParseRes.err != nil {
		return nil, fileParseRes.err
	}

	// 收集所有检索到的文档
	var documents []*schema.Document
	if retrievalRes.documents != nil {
		documents = retrievalRes.documents
		res.References = retrievalRes.documents
	}

	// 4. 如果启用MCP，先进行MCP工具调用（在LLM生成答案之前）
	var mcpDocs []*schema.Document
	if req.UseMCP {
		g.Log().Infof(ctx, "Checking if MCP tools are needed...")
		mcpHandler := NewMCPHandler()

		// 4.1 检查是否需要进行工具选择
		// 如果没有传入工具列表，或者工具数量超过12个，则使用LLM进行工具选择
		if req.MCPServiceTools == nil || len(req.MCPServiceTools) == 0 || h.countTotalTools(req.MCPServiceTools) > 12 {
			g.Log().Infof(ctx, "工具列表为空或超过20个，使用LLM进行工具选择")

			// 构建用于工具选择的完整问题（包含检索内容和文件内容）
			toolSelectionQuestion := h.buildToolSelectionQuestion(ctx, req.Question, documents, fileParseRes.fileContent)

			// 使用LLM选择工具
			selectedTools, selectErr := h.selectToolsWithLLM(ctx, toolSelectionQuestion)
			if selectErr != nil {
				g.Log().Errorf(ctx, "工具选择失败: %v", selectErr)
				// 工具选择失败，使用原有的工具列表（如果为空则调用所有工具）
			} else {
				g.Log().Infof(ctx, "LLM选择了 %d 个服务的工具", len(selectedTools))
				// 更新请求中的工具列表
				req.MCPServiceTools = selectedTools
			}
		}

		// 4.2 执行MCP工具调用，传入知识检索和文件解析的结果
		var mcpResults []*v1.MCPResult
		var mcpFinalAnswer string
		var mcpErr error
		mcpDocs, mcpResults, mcpFinalAnswer, mcpErr = mcpHandler.CallMCPToolsWithLLM(ctx, req, documents, fileParseRes.fileContent)
		if mcpErr != nil {
			g.Log().Errorf(ctx, "MCP tool call failed: %v", mcpErr)
		} else {
			// 如果 MCP 返回了最终答案，直接使用它
			if mcpFinalAnswer != "" {
				g.Log().Infof(ctx, "MCP returned final answer, skipping additional LLM call")
				res.Answer = mcpFinalAnswer
				res.MCPResults = mcpResults

				// 将MCP返回的文档添加到references中（仅用于展示工具调用结果）
				if len(mcpDocs) > 0 {
					res.References = append(res.References, mcpDocs...)
				}

				return res, nil
			}

			// 如果没有最终答案但有工具调用结果，添加到documents中供后续LLM使用
			if len(mcpResults) > 0 {
				g.Log().Infof(ctx, "MCP tools returned %d results", len(mcpResults))
				res.MCPResults = mcpResults

				// 将MCP返回的文档添加到documents中，供后续LLM调用使用
				if len(mcpDocs) > 0 {
					documents = append(documents, mcpDocs...)
					res.References = append(res.References, mcpDocs...)
				}
			}
		}
	}

	// 5. 调用Chat逻辑生成最终答案（仅在MCP未返回最终答案时）
	chatI := chat.GetChat()

	// 根据是否有文件或文档内容选择不同的处理方式
	if len(fileParseRes.multimodalFiles) > 0 || fileParseRes.fileContent != "" || len(fileParseRes.fileImages) > 0 {
		// 有文件或文档内容：使用文件对话模式
		g.Log().Infof(ctx, "Using file-based chat with %d multimodal files, text content length: %d, %d images",
			len(fileParseRes.multimodalFiles), len(fileParseRes.fileContent), len(fileParseRes.fileImages))
		answer, reasoningContent, err := chatI.GetAnswerWithParsedFiles(ctx, req.ModelID, req.ConvID, documents, req.Question,
			fileParseRes.multimodalFiles, fileParseRes.fileContent, fileParseRes.fileImages, req.JsonFormat)
		if err != nil {
			return nil, err
		}
		res.Answer = answer
		res.ReasoningContent = reasoningContent
	} else {
		// 无文件：普通对话模式
		g.Log().Infof(ctx, "Using standard chat without files")
		answer, reasoningContent, err := chatI.GetAnswer(ctx, req.ModelID, req.ConvID, documents, req.Question, req.JsonFormat)
		if err != nil {
			return nil, err
		}
		res.Answer = answer
		res.ReasoningContent = reasoningContent
	}

	return res, nil
}
