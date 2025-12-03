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

	// 4. 调用Chat逻辑生成答案
	chatI := chat.GetChat()

	var answer string
	var err error

	// 根据是否有文件或文档内容选择不同的处理方式
	if len(fileParseRes.multimodalFiles) > 0 || fileParseRes.fileContent != "" || len(fileParseRes.fileImages) > 0 {
		// 有文件或文档内容：使用文件对话模式
		g.Log().Infof(ctx, "Using file-based chat with %d multimodal files, text content length: %d, %d images",
			len(fileParseRes.multimodalFiles), len(fileParseRes.fileContent), len(fileParseRes.fileImages))
		answer, err = chatI.GetAnswerWithParsedFiles(ctx, req.ModelID, req.ConvID, documents, req.Question,
			fileParseRes.multimodalFiles, fileParseRes.fileContent, fileParseRes.fileImages)
	} else {
		// 无文件：普通对话模式
		g.Log().Infof(ctx, "Using standard chat without files")
		answer, err = chatI.GetAnswer(ctx, req.ModelID, req.ConvID, documents, req.Question)
	}

	if err != nil {
		return nil, err
	}

	res.Answer = answer

	// 5. 如果启用MCP，进行MCP工具调用（单次调用）
	if req.UseMCP {
		g.Log().Infof(ctx, "Checking if MCP tools are needed...")
		mcpHandler := NewMCPHandler()
		mcpDocs, mcpResults, mcpErr := mcpHandler.CallMCPToolsWithLLM(ctx, req)
		if mcpErr != nil {
			g.Log().Errorf(ctx, "MCP tool call failed: %v", mcpErr)
		} else if len(mcpResults) > 0 {
			// 如果MCP返回了结果，需要整合到答案中
			g.Log().Infof(ctx, "MCP tools returned %d results, integrating into answer", len(mcpResults))
			res.MCPResults = mcpResults

			// 将MCP返回的文档也添加到references中
			if len(mcpDocs) > 0 {
				res.References = append(res.References, mcpDocs...)
			}

			// TODO: 可以选择是否用MCP结果重新生成答案
			// 这里保持简单，只返回MCP结果供前端展示
		}
	}

	return res, nil
}
