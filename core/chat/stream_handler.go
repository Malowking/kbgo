package chat

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent_tools"
	"github.com/Malowking/kbgo/core/agent_tools/builtin"
	"github.com/Malowking/kbgo/core/agent_tools/executor"
	"github.com/Malowking/kbgo/core/agent_tools/executor/mcp"
	"github.com/Malowking/kbgo/core/agent_tools/registry"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/schema"
	"github.com/Malowking/kbgo/internal/history"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/logic/retriever"
	"github.com/Malowking/kbgo/internal/logic/rewriter"
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
	// 初始化历史管理器
	historyManager := history.NewManager()

	// 先获取历史记录（不包含当前用户消息）
	fullChatHistory, err := historyManager.GetHistory(req.ConvID, 50)
	if err != nil {
		g.Log().Warningf(ctx, "Failed to get chat history: %v", err)
		fullChatHistory = []*schema.Message{}
	}
	g.Log().Infof(ctx, "Retrieved %d messages from history for processing", len(fullChatHistory))

	// 保存当前用户消息到数据库（异步保存，不影响后续流程）
	if req.ConvID != "" {
		userMessageTime := time.Now()
		userMessage := &schema.Message{
			Role:    schema.User,
			Content: req.Question,
		}

		if err := historyManager.SaveMessage(userMessage, req.ConvID, nil, &userMessageTime); err != nil {
			g.Log().Warningf(ctx, "保存用户消息失败: %v，继续执行", err)
		} else {
			g.Log().Infof(ctx, "成功保存用户消息")
		}
	}

	// 从完整历史中截取最近10条用于查询重写
	var rewriteHistory []*schema.Message
	if len(fullChatHistory) > 10 {
		rewriteHistory = fullChatHistory[len(fullChatHistory)-10:]
	} else {
		rewriteHistory = fullChatHistory
	}

	// 查询重写
	queryRewriter := rewriter.NewQueryRewriter()
	rewriteConfig := rewriter.DefaultConfig()
	rewriteConfig.Enable = true
	rewriteConfig.ModelID = req.ModelID

	rewrittenQuery, err := queryRewriter.RewriteQuery(ctx, req.Question, rewriteHistory, rewriteConfig)
	if err != nil {
		g.Log().Warningf(ctx, "Query rewrite failed: %v, using original query", err)
		rewrittenQuery = req.Question
	} else if rewrittenQuery != req.Question {
		g.Log().Infof(ctx, "Query rewritten: [%s] -> [%s]", req.Question, rewrittenQuery)
	}

	// 获取检索配置
	cfg := retriever.GetRetrieverConfig()

	// 执行知识检索
	type retrievalResult struct {
		documents         []*schema.Document
		retrieverMetadata map[string]interface{}
		err               error
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

			retrieverRes, err := retriever.ProcessRetrieval(ctx, &v1.RetrieverReq{
				Question:        rewrittenQuery,
				RerankModelID:   req.RerankModelID,
				TopK:            req.TopK,
				Score:           req.Score,
				KnowledgeId:     req.KnowledgeId,
				EnableRewrite:   true,
				RewriteAttempts: 3,
				RetrieveMode:    retrieveMode,
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
	var retrievalDocuments []*schema.Document
	retrievalDocuments = retrievalRes.documents

	// 如果有解析的文档内容，添加到 retrievalDocuments 中
	if fileParseRes.fileContent != "" {
		retrievalDocuments = append(retrievalDocuments, &schema.Document{
			ID:       "uploaded_document",
			Content:  fileParseRes.fileContent,
			MetaData: map[string]interface{}{"source": "user_upload", "type": "document"},
		})
		g.Log().Infof(ctx, "Added parsed document content to documents (%d chars)", len(fileParseRes.fileContent))
	}

	// 返回的文档
	var allDocumentsForLLM []*schema.Document
	allDocumentsForLLM = append(allDocumentsForLLM, retrievalDocuments...)

	// 工具执行
	var toolResults []*schema.ToolResult
	var toolMessages []*schema.Message
	var planMessageID string
	var eventMgr *StreamEventManager
	var toolCallsToSave []schema.ToolCall
	var preRegisteredToolRegistry registry.ToolRegistry
	if req.Tools != nil && len(req.Tools) > 0 {
		g.Log().Infof(ctx, "Using intelligent tool execution with plan generation")

		// 创建流式事件管理器
		eventMgr = NewStreamEventManager(ctx)
		if eventMgr == nil {
			return fmt.Errorf("工具事件管理器创建失败")
		}

		// 创建工具管理器（统一管理所有工具）
		toolManager := agent_tools.NewToolManager()

		// 创建工具计划生成器
		planGenerator := executor.NewToolPlanGenerator(req.ModelID)

		// 检查是否有MCP工具配置，如果有则初始化MCP执行器并同时获取和注册工具
		for _, toolConfig := range req.Tools {
			if toolConfig.Enabled && toolConfig.Type == "mcp" {
				// 初始化MCP执行器
				mcpToolCaller, err := toolManager.SetMCPExecutor(ctx)
				if err != nil {
					g.Log().Errorf(ctx, "Failed to initialize MCP executor: %v", err)
				} else {
					// 创建工具注册表（用于同时注册MCP工具）
					preRegisteredToolRegistry = registry.NewToolDefinitionRegistry()

					// 获取MCP工具列表（根据用户配置过滤）
					var serviceToolsFilter map[string][]string
					if toolConfig.Config != nil {
						if filter, ok := toolConfig.Config["service_tools_filter"].(map[string][]string); ok {
							serviceToolsFilter = filter
						}
					}

					mcpTools, err := mcpToolCaller.GetFilteredLLMToolsAndRegister(ctx, serviceToolsFilter, preRegisteredToolRegistry)
					if err != nil {
						g.Log().Errorf(ctx, "Failed to get and register MCP tools: %v", err)
					} else {
						planGenerator.SetMCPTools(mcpTools)
						g.Log().Infof(ctx, "Initialized and registered MCP executor with %d tools", len(mcpTools))
					}
				}
				break
			}
		}

		// 生成工具执行计划
		plan, err := planGenerator.GeneratePlan(
			ctx,
			req.Question,
			rewrittenQuery,
			req.Tools,
			req.SystemPrompt,
			fullChatHistory,
			eventMgr,
		)

		if err != nil {
			g.Log().Errorf(ctx, "Failed to generate tool execution plan: %v", err)
			return fmt.Errorf("工具计划生成失败: %w", err)
		}

		planMessageID = plan.MessageID
		// 记录计划
		planGenerator.LogPlan(ctx, plan)

		// 验证计划
		if err := planGenerator.ValidatePlan(plan, req.Tools); err != nil {
			g.Log().Errorf(ctx, "Plan validation failed: %v", err)
			return fmt.Errorf("工具计划验证失败: %w", err)
		}

		// 执行工具
		if plan.NeedTools && len(plan.Steps) > 0 {
			g.Log().Infof(ctx, "Executing tools with native tool calling")

			toolRegistry := initializeToolRegistry(ctx, req.Tools, toolManager, preRegisteredToolRegistry)

			// 创建顺序执行器
			seqExecutor := executor.NewSequentialToolExecutor(
				req.ModelID,
				toolManager,
				eventMgr,
				toolRegistry,
				historyManager,
				plan.MessageID,
			)

			// 执行工具
			var err error
			toolMessages, toolResults, err = seqExecutor.ExecuteToolsWithPlan(
				ctx,
				plan,
				req.Tools,
				req.ConvID,
				req.ModelID,
				rewrittenQuery,
			)

			if err != nil {
				g.Log().Errorf(ctx, "Tool execution failed: %v", err)
				return fmt.Errorf("工具执行失败: %w", err)
			}

			for _, msg := range toolMessages {
				if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
					toolCallsToSave = append(toolCallsToSave, msg.ToolCalls...)
				}
			}

			//g.Log().Infof(ctx, "Tool execution completed, got %d results", len(toolMessages))
			//// JSON格式化打印toolResults
			//if len(toolMessages) > 0 {
			//	for i, result := range toolMessages {
			//		resultJSON, err := json.Marshal(result)
			//		if err != nil {
			//			g.Log().Errorf(ctx, "Failed to marshal tool result %d to JSON: %v", i, err)
			//		} else {
			//			g.Log().Infof(ctx, "Tool message--------------------------- %d: %s", i, string(resultJSON))
			//		}
			//	}
			//}
		} else {
			g.Log().Infof(ctx, "No tools needed according to plan")
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

	// 准备消息列表
	var messages []*schema.Message
	if len(toolMessages) > 0 {
		g.Log().Infof(ctx, "Tool execution completed, appending %d tool messages to history", len(toolMessages))
		finalHistory := append(fullChatHistory, &schema.Message{
			Role:    schema.User,
			Content: req.Question,
		})
		finalHistory = append(finalHistory, toolMessages...)
		//
		//for i, result := range finalHistory {
		//	resultJSON, err := json.Marshal(result)
		//	if err != nil {
		//		g.Log().Errorf(ctx, "Failed to marshal tool result %d to JSON: %v", i, err)
		//	} else {
		//		g.Log().Infof(ctx, "---------finalHistory------------ %d: %s", i, string(resultJSON))
		//	}
		//}
		messages = chat.BuildMessagesForChat(ctx, req.SystemPrompt, finalHistory, retrievalDocuments)
	} else {
		g.Log().Infof(ctx, "No tool execution, using %d messages from history", len(fullChatHistory))
		messages = chat.BuildMessagesForChat(ctx, req.SystemPrompt, fullChatHistory, retrievalDocuments)
	}

	g.Log().Infof(ctx, "Final messages count: %d", len(messages))
	//for i, result := range messages {
	//	resultJSON, err := json.Marshal(result)
	//	if err != nil {
	//		g.Log().Errorf(ctx, "Failed to marshal tool result %d to JSON: %v", i, err)
	//	} else {
	//		g.Log().Infof(ctx, "============================Tool result %d: %s", i, string(resultJSON))
	//	}
	//}
	// 获取流式响应
	var streamReader schema.StreamReaderInterface[*schema.Message]
	if len(multimodalFiles) > 0 {
		g.Log().Infof(ctx, "Using multimodal stream chat with %d files", len(multimodalFiles))
		streamReader, err = chatI.GetAnswerStreamWithFiles(ctx, req.ModelID, req.ConvID, messages, req.Question, multimodalFiles, req.JsonFormat, planMessageID)
	} else {
		g.Log().Infof(ctx, "Calling GetAnswerStream with %d messages", len(messages))
		streamReader, err = chatI.GetAnswerStream(ctx, req.ModelID, req.ConvID, messages, req.JsonFormat, planMessageID)
	}
	if err != nil {
		g.Log().Error(ctx, err)
		return err
	}
	defer streamReader.Close()

	if eventMgr != nil && planMessageID != "" {
		_ = eventMgr.SendFinalAnswerStart(planMessageID)
	}

	// 处理流式响应和内容收集
	err = h.handleStreamResponse(ctx, streamReader, retrievalDocuments, toolResults, start, req.ConvID, retrievalRes.retrieverMetadata, chatI, planMessageID)
	if err != nil {
		g.Log().Error(ctx, err)
		return err
	}

	return nil
}

// handleStreamResponse 处理流式响应
func (h *StreamHandler) handleStreamResponse(ctx context.Context, streamReader schema.StreamReaderInterface[*schema.Message], retrievalDocuments []*schema.Document, toolResults []*schema.ToolResult, start time.Time, convID string, metadata map[string]interface{}, chatI interface{}, messageID string) error {
	// 直接发送流式响应到客户端
	err := common.SteamResponse(ctx, streamReader, retrievalDocuments, toolResults, messageID)
	if err != nil {
		return err
	}

	return nil
}

// initializeToolRegistry 初始化工具注册表
func initializeToolRegistry(ctx context.Context, toolConfigs []*v1.ToolConfig, toolManager *agent_tools.ToolManager, preRegisteredRegistry registry.ToolRegistry) registry.ToolRegistry {
	var toolRegistry registry.ToolRegistry

	if preRegisteredRegistry != nil {
		toolRegistry = preRegisteredRegistry
		g.Log().Infof(ctx, "[StreamHandler] Reusing pre-registered tool registry with MCP tools")
	} else {
		// 否则创建新的工具注册表
		toolRegistry = registry.NewToolDefinitionRegistry()
	}

	// 注册内置工具
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		g.Log().Errorf(ctx, "[StreamHandler] Failed to register builtin tools: %v", err)
	} else {
		g.Log().Infof(ctx, "[StreamHandler] Registered builtin tools")
	}

	// 如果没有预注册的 registry，则需要注册 MCP 工具
	if preRegisteredRegistry == nil {
		for _, toolConfig := range toolConfigs {
			if toolConfig.Enabled && toolConfig.Type == "mcp" {
				// 从 toolManager 获取 MCP 执行器
				mcpToolCaller := toolManager.GetMCPExecutor()
				if mcpToolCaller != nil {
					// 获取所有 MCP 客户端并注册工具
					clients := mcpToolCaller.GetAllClients()
					for serviceName, client := range clients {
						if err := mcp.RegisterMCPTools(ctx, toolRegistry, client, serviceName); err != nil {
							g.Log().Errorf(ctx, "[StreamHandler] Failed to register MCP tools from %s: %v", serviceName, err)
						} else {
							g.Log().Infof(ctx, "[StreamHandler] Registered MCP tools from service: %s", serviceName)
						}
					}
				} else {
					g.Log().Warningf(ctx, "[StreamHandler] MCP executor not initialized, skipping MCP tools registration")
				}
				break
			}
		}
	} else {
		g.Log().Infof(ctx, "[StreamHandler] Skipping MCP tools registration (already registered in pre-registered registry)")
	}

	return toolRegistry
}
