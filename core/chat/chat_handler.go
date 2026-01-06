package chat

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent_tools"
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

// Chat Handle basic chat request (non-streaming)
func (h *ChatHandler) Chat(ctx context.Context, req *v1.ChatReq, uploadedFiles []*common.MultimodalFile) (*v1.ChatRes, error) {
	// 验证：Tools和知识库检索不能同时启用
	if req.Tools != nil && len(req.Tools) > 0 && req.EnableRetriever && req.KnowledgeId != "" {
		g.Log().Warningf(ctx, "Chat handler - Both Tools and knowledge retrieval are enabled, knowledge retrieval will be treated as a tool")
		// 当Tools参数存在时，知识库检索会作为工具的一部分处理，所以这里禁用直接的知识库检索
		req.EnableRetriever = false
	}

	// 加载Agent预设配置（如果会话关联了Agent预设）
	req = LoadAgentPresetConfig(ctx, req)

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
				RerankWeight:     req.RerankWeight,
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

	// 2. 并行处理文件
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

	// 4. 执行工具调用 (使用统一的工具执行器)
	if req.Tools != nil && len(req.Tools) > 0 {
		g.Log().Infof(ctx, "Executing tools using unified executor with LLM selection")
		executor := agent_tools.NewToolExecutor()

		// 传入 SystemPrompt 和 ConvID，让工具选择阶段也能感知 Agent 的角色
		toolResult, err := executor.Execute(ctx, req.Tools, req.Question,
			req.ModelID, req.EmbeddingModelID, documents, req.SystemPrompt, req.ConvID)

		if err != nil {
			g.Log().Errorf(ctx, "Tool execution failed: %v", err)
		} else {
			// 添加工具返回的文档
			if len(toolResult.Documents) > 0 {
				documents = append(documents, toolResult.Documents...)
				res.References = append(res.References, toolResult.Documents...)
			}

			// 设置NL2SQL结果
			if toolResult.NL2SQLResult != nil {
				res.NL2SQLResult = toolResult.NL2SQLResult
			}

			// 设置MCP结果
			if toolResult.MCPResults != nil {
				res.MCPResults = toolResult.MCPResults
			}

			// 如果工具返回了最终答案,直接返回
			if toolResult.FinalAnswer != "" {
				g.Log().Infof(ctx, "Tool returned final answer, skipping LLM call")
				res.Answer = toolResult.FinalAnswer

				// 转换返回的文档中的图片URL为可访问的代理URL
				r := g.RequestFromCtx(ctx)
				if r != nil && res.References != nil {
					baseURL := common.GetBaseURL(r.Host, r.URL.Scheme, map[string]string{
						"X-Forwarded-Host":  r.Header.Get("X-Forwarded-Host"),
						"X-Forwarded-Proto": r.Header.Get("X-Forwarded-Proto"),
					})
					common.ConvertImageURLsInDocuments(res.References, baseURL)
				}

				return res, nil
			}
		}
	}

	// 5. 调用Chat逻辑生成最终答案
	chatI := chat.GetChat()

	// 根据是否有文件或文档内容选择不同的处理方式
	if len(fileParseRes.multimodalFiles) > 0 || fileParseRes.fileContent != "" || len(fileParseRes.fileImages) > 0 {
		// 有文件或文档内容：使用文件对话模式
		g.Log().Infof(ctx, "Using file-based chat with %d multimodal files, text content length: %d, %d images",
			len(fileParseRes.multimodalFiles), len(fileParseRes.fileContent), len(fileParseRes.fileImages))
		answer, reasoningContent, err := chatI.GetAnswerWithParsedFiles(ctx, req.ModelID, req.ConvID, documents, req.Question,
			fileParseRes.multimodalFiles, fileParseRes.fileContent, fileParseRes.fileImages, req.SystemPrompt, req.JsonFormat)
		if err != nil {
			return nil, err
		}
		res.Answer = answer
		res.ReasoningContent = reasoningContent
	} else {
		// 无文件：普通对话模式
		g.Log().Infof(ctx, "Using standard chat without files")
		answer, reasoningContent, err := chatI.GetAnswer(ctx, req.ModelID, req.ConvID, documents, req.Question, req.SystemPrompt, req.JsonFormat)
		if err != nil {
			return nil, err
		}
		res.Answer = answer
		res.ReasoningContent = reasoningContent
	}

	// 转换返回的文档中的图片URL为可访问的代理URL
	r := g.RequestFromCtx(ctx)
	if r != nil && res.References != nil {
		baseURL := common.GetBaseURL(r.Host, r.URL.Scheme, map[string]string{
			"X-Forwarded-Host":  r.Header.Get("X-Forwarded-Host"),
			"X-Forwarded-Proto": r.Header.Get("X-Forwarded-Proto"),
		})
		common.ConvertImageURLsInDocuments(res.References, baseURL)
	}

	return res, nil
}
