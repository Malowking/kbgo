package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Malowking/kbgo/api/rag/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/chat"
	rag2 "github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/Malowking/kbgo/internal/mcp/client"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	// 如果启用流式返回，执行流式逻辑
	if req.Stream {
		return nil, c.handleStreamChat(ctx, req)
	}

	// 获取检索配置
	cfg := rag2.GetRetrieverConfig()

	// 初始化返回结果
	res = &v1.ChatRes{}

	// 如果启用了知识库检索且提供了知识库ID，则进行检索
	var documents []*schema.Document
	if req.EnableRetriever && req.KnowledgeId != "" {
		retriever, err := c.Retriever(ctx, &v1.RetrieverReq{
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
		mcpDocs, mcpRes, err := c.callMCPToolsWithLLM(ctx, req)
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

// handleStreamChat 处理流式聊天请求
func (c *ControllerV1) handleStreamChat(ctx context.Context, req *v1.ChatReq) error {
	var streamReader *schema.StreamReader[*schema.Message]

	// 获取检索配置
	cfg := rag2.GetRetrieverConfig()

	// 如果启用了知识库检索且提供了知识库ID，则进行检索
	var documents []*schema.Document
	var retrieverMetadata map[string]interface{}
	if req.EnableRetriever && req.KnowledgeId != "" {
		retriever, err := c.Retriever(ctx, &v1.RetrieverReq{
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
		mcpDocs, mcpRes, err := c.callMCPToolsWithLLM(ctx, req)
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

	// 准备元数据
	metadata := map[string]interface{}{}
	if retrieverMetadata != nil {
		metadata["retriever"] = retrieverMetadata
	}
	if mcpMetadata != nil {
		metadata["mcp_tools"] = mcpMetadata
	}

	// 将元数据添加到所有文档中
	if len(metadata) > 0 {
		for _, doc := range allDocuments {
			if doc.MetaData == nil {
				doc.MetaData = make(map[string]interface{})
			}
			doc.MetaData["chat_metadata"] = metadata
		}
	}

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
			chatI.SaveStreamingMessageWithMetadata(req.ConvID, fullMessage, metadata)
		}
	}()

	err = common.SteamResponse(ctx, streamReader, allDocuments)
	if err != nil {
		g.Log().Error(ctx, err)
		return err
	}

	return nil
}

// extractMCPResults 从MCP文档中提取结果信息
func (c *ControllerV1) extractMCPResults(docs []*schema.Document) []*v1.MCPResult {
	var results []*v1.MCPResult
	for _, doc := range docs {
		if source, ok := doc.MetaData["source"].(string); ok && source == "mcp" {
			result := &v1.MCPResult{
				ServiceName: "",
				ToolName:    "",
				Content:     doc.Content,
			}

			if serviceName, ok := doc.MetaData["service"].(string); ok {
				result.ServiceName = serviceName
			}
			if toolName, ok := doc.MetaData["tool"].(string); ok {
				result.ToolName = toolName
			}
			results = append(results, result)
		}
	}
	return results
}

// callMCPTools 调用MCP工具
func (c *ControllerV1) callMCPTools(ctx context.Context, req *v1.ChatReq) ([]*schema.Document, error) {
	g.Log().Debugf(ctx, "开始调用MCP工具, UseMCP: %v, MCPServiceTools: %v", req.UseMCP, req.MCPServiceTools)

	// 获取所有启用的MCP服务
	registries, _, err := dao.MCPRegistry.List(ctx, nil, 1, 100)
	if err != nil {
		g.Log().Errorf(ctx, "获取MCP服务列表失败: %v", err)
		return nil, err
	}

	g.Log().Debugf(ctx, "获取到MCP服务数量: %d", len(registries))

	// 如果没有注册的服务，直接返回
	if len(registries) == 0 {
		g.Log().Debug(ctx, "没有找到注册的MCP服务")
		return nil, nil
	}

	var documents []*schema.Document

	// 遍历所有MCP服务
	for _, registry := range registries {
		g.Log().Debugf(ctx, "检查MCP服务: %s, 状态: %d", registry.Name, registry.Status)

		// 检查服务是否启用
		if registry.Status != 1 {
			g.Log().Debugf(ctx, "MCP服务 %s 未启用，跳过", registry.Name)
			continue
		}

		// 创建客户端
		mcpClient := client.NewMCPClient(registry)
		g.Log().Debugf(ctx, "创建MCP客户端: %s", registry.Name)

		// 初始化连接
		err = mcpClient.Initialize(ctx, map[string]interface{}{
			"name":    "kbgo",
			"version": "1.0.0",
		})
		if err != nil {
			g.Log().Errorf(ctx, "Failed to initialize MCP connection for service %s: %v", registry.Name, err)
			continue
		}
		g.Log().Debugf(ctx, "初始化MCP连接成功: %s", registry.Name)

		// 获取工具列表
		var tools []client.MCPTool
		// 首先尝试从数据库缓存中获取工具列表
		if registry.Tools != "" && registry.Tools != "[]" {
			var toolInfos []v1.MCPToolInfo
			if err := json.Unmarshal([]byte(registry.Tools), &toolInfos); err == nil {
				// 转换为client.MCPTool格式
				tools = make([]client.MCPTool, len(toolInfos))
				for i, info := range toolInfos {
					tools[i] = client.MCPTool{
						Name:        info.Name,
						Description: info.Description,
						InputSchema: info.InputSchema,
					}
				}
				g.Log().Debugf(ctx, "从数据库缓存获取到MCP工具数量: %d", len(tools))
			}
		}

		// 如果缓存中没有工具列表，则从远程服务获取
		if len(tools) == 0 {
			tools, err = mcpClient.ListTools(ctx)
			if err != nil {
				g.Log().Errorf(ctx, "Failed to list MCP tools for service %s: %v", registry.Name, err)
				continue
			}

			// 更新数据库中的工具列表缓存
			if len(tools) > 0 {
				// 转换为MCPToolInfo格式
				toolInfos := make([]v1.MCPToolInfo, len(tools))
				for i, tool := range tools {
					toolInfos[i] = v1.MCPToolInfo{
						Name:        tool.Name,
						Description: tool.Description,
						InputSchema: tool.InputSchema,
					}
				}

				// 序列化并保存到数据库
				toolsJSON, err := json.Marshal(toolInfos)
				if err == nil {
					registry.Tools = string(toolsJSON)
					if updateErr := dao.MCPRegistry.Update(ctx, registry); updateErr != nil {
						g.Log().Errorf(ctx, "Failed to update MCP registry tools: %v", updateErr)
					}
				}
			}
		}

		g.Log().Debugf(ctx, "获取到MCP工具数量: %d, 工具列表: %v", len(tools), tools)

		// 遍历工具并调用符合条件的工具
		for _, tool := range tools {
			g.Log().Debugf(ctx, "检查工具: %s", tool.Name)

			// 处理工具调用逻辑：
			// 1. 如果MCPServiceTools指定了该服务的工具，则只调用指定的工具
			// 2. 如果MCPServiceTools为空或nil，调用所有工具

			if req.MCPServiceTools != nil {
				// 检查是否为特定服务指定了工具
				if serviceTools, exists := req.MCPServiceTools[registry.Name]; exists {
					g.Log().Debugf(ctx, "检查服务 %s 的指定工具列表: %v", registry.Name, serviceTools)
					if len(serviceTools) == 0 {
						// 空数组表示不调用该服务的任何工具
						g.Log().Debugf(ctx, "服务 %s 的工具列表为空，跳过所有工具", registry.Name)
						continue
					}

					// 检查工具是否在允许列表中
					found := false
					for i, allowedTool := range serviceTools {
						g.Log().Debugf(ctx, "比较工具名称: 索引%d, '%s' vs '%s'", i, allowedTool, tool.Name)
						if allowedTool == tool.Name {
							found = true
							g.Log().Debugf(ctx, "找到匹配的工具: %s", tool.Name)
							break
						}
					}
					if !found {
						g.Log().Debugf(ctx, "工具 %s 不在服务 %s 的允许列表中，跳过", tool.Name, registry.Name)
						continue
					}
				}
			} else {
				g.Log().Debug(ctx, "未指定工具列表，调用所有工具")
			}

			g.Log().Debugf(ctx, "调用MCP工具: %s", tool.Name)

			startTime := time.Now()

			// 智能参数映射：根据工具schema生成参数
			toolArgs, err := c.buildToolArguments(tool, req.Question)
			if err != nil {
				g.Log().Warningf(ctx, "构建工具参数失败: %v", err)
				// 使用fallback策略
				toolArgs = c.fallbackToolArguments(tool, req.Question)
			}

			// 调用工具
			result, err := mcpClient.CallTool(ctx, tool.Name, toolArgs)

			// 计算耗时
			duration := int(time.Since(startTime).Milliseconds())

			// 序列化请求和响应
			reqPayload, _ := json.Marshal(toolArgs)
			respPayload, _ := json.Marshal(result)

			// 记录调用日志
			logStatus := int8(1) // 成功
			errorMsg := ""
			if err != nil {
				logStatus = 0 // 失败
				errorMsg = err.Error()
			}

			logID := "log_" + strings.ReplaceAll(uuid.New().String(), "-", "")
			callLog := &gormModel.MCPCallLog{
				ID:              logID,
				ConversationID:  req.ConvID,
				MCPRegistryID:   registry.ID,
				MCPServiceName:  registry.Name,
				ToolName:        tool.Name,
				RequestPayload:  string(reqPayload),
				ResponsePayload: string(respPayload),
				Status:          logStatus,
				ErrorMessage:    errorMsg,
				Duration:        duration,
			}

			if logErr := dao.MCPCallLog.Create(ctx, callLog); logErr != nil {
				g.Log().Errorf(ctx, "Failed to create MCP call log: %v", logErr)
			}

			// 如果调用失败，记录错误并继续
			if err != nil {
				g.Log().Errorf(ctx, "Failed to call MCP tool %s: %v", tool.Name, err)
				continue
			}

			g.Log().Debugf(ctx, "MCP工具调用成功: %s, 结果: %v", tool.Name, result)

			// 将结果转换为文档
			for _, content := range result.Content {
				if content.Type == "text" && content.Text != "" {
					doc := &schema.Document{
						ID:      logID,
						Content: content.Text,
						MetaData: map[string]interface{}{
							"source":    "mcp",
							"service":   registry.Name,
							"tool":      tool.Name,
							"tool_desc": tool.Description,
						},
					}
					documents = append(documents, doc)
				}
			}
		}
	}

	g.Log().Debugf(ctx, "MCP工具调用完成，返回文档数量: %d", len(documents))

	return documents, nil
}

// buildToolArguments 根据工具schema智能构建参数
func (c *ControllerV1) buildToolArguments(tool client.MCPTool, question string) (map[string]interface{}, error) {
	args := make(map[string]interface{})

	// 解析工具的输入schema
	if tool.InputSchema == nil {
		return c.fallbackToolArguments(tool, question), nil
	}

	// 获取properties
	properties, ok := tool.InputSchema["properties"]
	if !ok {
		return c.fallbackToolArguments(tool, question), nil
	}

	propertiesMap, ok := properties.(map[string]interface{})
	if !ok {
		return c.fallbackToolArguments(tool, question), nil
	}

	// 遍历每个参数并尝试映射
	for paramName, paramDef := range propertiesMap {
		value := c.mapParameterValue(paramName, paramDef, question)
		if value != nil {
			args[paramName] = value
		}
	}

	// 如果没有成功映射任何参数，使用fallback策略
	if len(args) == 0 {
		return c.fallbackToolArguments(tool, question), nil
	}

	return args, nil
}

// mapParameterValue 根据参数名和类型映射值
func (c *ControllerV1) mapParameterValue(paramName string, paramDef interface{}, question string) interface{} {
	// 将参数名转换为小写进行匹配
	lowerParamName := strings.ToLower(paramName)

	// 根据参数名智能映射
	switch {
	case strings.Contains(lowerParamName, "name"):
		// 尝试从问题中提取人名
		if extractedName := c.extractNameFromQuestion(question); extractedName != "" {
			return extractedName
		}
		return question // fallback
	case strings.Contains(lowerParamName, "question") || strings.Contains(lowerParamName, "query"):
		return question
	case strings.Contains(lowerParamName, "text") || strings.Contains(lowerParamName, "content"):
		return question
	case strings.Contains(lowerParamName, "message") || strings.Contains(lowerParamName, "msg"):
		return question
	default:
		// 对于其他类型的参数，尝试基于类型设置默认值
		paramDefMap, ok := paramDef.(map[string]interface{})
		if !ok {
			return question
		}

		paramType, exists := paramDefMap["type"]
		if !exists {
			return question
		}

		switch paramType {
		case "string":
			return question
		case "boolean":
			return true // 默认true
		case "number", "integer":
			return 1 // 默认1
		default:
			return question
		}
	}
}

// extractNameFromQuestion 尝试从问题中提取姓名
func (c *ControllerV1) extractNameFromQuestion(question string) string {
	// 简单的姓名提取逻辑
	question = strings.TrimSpace(question)

	// 如果问题很短且看起来像名字，直接返回
	if len(question) <= 20 && !strings.Contains(question, " ") {
		// 排除一些明显不是名字的词
		lowQuestion := strings.ToLower(question)
		if !strings.Contains(lowQuestion, "how") &&
			!strings.Contains(lowQuestion, "what") &&
			!strings.Contains(lowQuestion, "why") &&
			!strings.Contains(lowQuestion, "when") &&
			!strings.Contains(lowQuestion, "where") {
			return question
		}
	}

	// 查找常见的姓名模式
	words := strings.Fields(question)
	for _, word := range words {
		// 如果单词是大写开头且长度适中，可能是名字
		if len(word) >= 2 && len(word) <= 15 && strings.Title(word) == word {
			// 排除一些常见的非名字单词
			lowWord := strings.ToLower(word)
			if lowWord != "hello" && lowWord != "hi" && lowWord != "the" && lowWord != "my" {
				return word
			}
		}
	}

	return ""
}

// fallbackToolArguments 提供fallback参数映射策略
func (c *ControllerV1) fallbackToolArguments(tool client.MCPTool, question string) map[string]interface{} {
	// 常见的参数名映射策略
	commonMappings := []string{"question", "query", "text", "content", "message", "input", "name"}

	args := make(map[string]interface{})

	// 尝试每种常见的参数名
	for _, paramName := range commonMappings {
		if paramName == "name" {
			// 特殊处理name参数
			if extractedName := c.extractNameFromQuestion(question); extractedName != "" {
				args[paramName] = extractedName
			} else {
				args[paramName] = question // fallback to question
			}
		} else {
			args[paramName] = question
		}
	}

	return args
}

// callMCPToolsWithLLM 使用 LLM 智能选择并调用 MCP 工具
func (c *ControllerV1) callMCPToolsWithLLM(ctx context.Context, req *v1.ChatReq) ([]*schema.Document, []*v1.MCPResult, error) {
	g.Log().Debugf(ctx, "开始LLM智能工具调用, 问题: %s", req.Question)

	// 创建 MCP 工具调用器
	toolCaller, err := rag2.NewMCPToolCaller(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("创建MCP工具调用器失败: %w", err)
	}
	defer toolCaller.Close()

	// 使用 LLM 智能选择并调用工具
	// 传递 MCPServiceTools 作为过滤器，限制 LLM 只能选择指定的工具
	documents, mcpResults, err := toolCaller.CallToolsWithLLM(ctx, req.Question, req.ConvID, req.MCPServiceTools)
	if err != nil {
		return nil, nil, fmt.Errorf("LLM智能工具调用失败: %w", err)
	}

	g.Log().Debugf(ctx, "LLM智能工具调用完成，返回文档数量: %d, MCP结果数量: %d", len(documents), len(mcpResults))

	return documents, mcpResults, nil
}

// callSingleTool 调用单个工具
func (tc *ControllerV1) callSingleTool(ctx context.Context, serviceName string, toolName string, args map[string]interface{}, convID string) (*schema.Document, *v1.MCPResult, error) {
	// 获取MCP服务
	registry, err := dao.MCPRegistry.GetByName(ctx, serviceName)
	if err != nil {
		return nil, nil, fmt.Errorf("获取MCP服务失败: %w", err)
	}

	// 创建客户端
	mcpClient := client.NewMCPClient(registry)

	// 初始化连接
	err = mcpClient.Initialize(ctx, map[string]interface{}{
		"name":    "kbgo",
		"version": "1.0.0",
	})
	if err != nil {
		return nil, nil, fmt.Errorf("初始化MCP连接失败: %w", err)
	}

	// 调用工具
	result, err := mcpClient.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, nil, fmt.Errorf("调用工具失败: %w", err)
	}

	// 提取文本内容
	var content string
	for _, c := range result.Content {
		if c.Type == "text" && c.Text != "" {
			content += c.Text + "\n"
		}
	}
	content = strings.TrimSpace(content)

	// 将结果转换为文档
	doc := &schema.Document{
		ID:      "mcp_" + serviceName + "_" + toolName,
		Content: content,
		MetaData: map[string]interface{}{
			"source":    "mcp",
			"service":   serviceName,
			"tool":      toolName,
			"tool_desc": "", // result.Description 不存在
		},
	}

	// 将结果转换为MCPResult
	mcpResult := &v1.MCPResult{
		ServiceName: serviceName,
		ToolName:    toolName,
		Content:     content,
	}

	return doc, mcpResult, nil
}

// callMCPToolsWithLLMAndSave 使用 LLM 智能选择并调用 MCP 工具，并保存消息历史
func (tc *ControllerV1) callMCPToolsWithLLMAndSave(ctx context.Context, convID string, messages []*schema.Message, llmTools []*schema.ToolInfo) ([]*schema.Document, []*v1.MCPResult, error) {
	// 1. 创建 MCP 工具调用器
	toolCaller, err := rag2.NewMCPToolCaller(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("创建MCP工具调用器失败: %w", err)
	}
	defer toolCaller.Close()

	// 2. 保存初始消息历史
	chatInstance := chat.GetChat()
	// 注意：这里没有 SaveMessages 方法，我们会在 GenerateWithToolsAndSave 中保存响应

	// 3. 调用 LLM（最多循环 5 次以支持多轮工具调用）
	maxIterations := 5
	var allDocuments []*schema.Document
	var allMCPResults []*v1.MCPResult
	var finalAnswer string                    // 保存 LLM 的最终文本回答
	var toolCallLogs []map[string]interface{} // 记录工具调用日志

	for iteration := 0; iteration < maxIterations; iteration++ {
		g.Log().Debugf(ctx, "====== 工具调用迭代 %d/%d ======", iteration+1, maxIterations)

		// 调用 LLM
		response, err := chatInstance.GenerateWithToolsAndSave(ctx, messages, llmTools)
		if err != nil {
			return nil, nil, fmt.Errorf("LLM 调用失败: %w", err)
		}

		g.Log().Debugf(ctx, "LLM 响应 - Content: %s, ToolCalls数量: %d", response.Content, len(response.ToolCalls))

		// 将 LLM 响应添加到消息历史
		messages = append(messages, response)

		// 注意：GenerateWithToolsAndSave已经保存了消息，这里不需要再保存

		// 4. 检查是否有工具调用
		if len(response.ToolCalls) == 0 {
			// 没有工具调用，LLM 已经给出最终答案
			finalAnswer = response.Content
			g.Log().Infof(ctx, "LLM 未调用任何工具，给出最终答案（长度: %d）", len(finalAnswer))
			break
		}

		// 5. 执行所有工具调用
		g.Log().Infof(ctx, "LLM 要求调用 %d 个工具", len(response.ToolCalls))

		for idx, toolCall := range response.ToolCalls {
			// 解析工具名（格式：serviceName__toolName）
			serviceName, toolName := client.ParseToolName(toolCall.Function.Name)
			g.Log().Debugf(ctx, "[工具 %d/%d] 调用: %s (服务: %s, 工具: %s)",
				idx+1, len(response.ToolCalls), toolCall.Function.Name, serviceName, toolName)

			// 解析参数
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				errMsg := fmt.Sprintf("参数解析错误: %v", err)
				g.Log().Errorf(ctx, "[工具 %d/%d] %s", idx+1, len(response.ToolCalls), errMsg)

				// 添加错误响应到消息历史
				messages = append(messages, &schema.Message{
					Role:       schema.Tool, // 注意：这里应该是 Tool 而不是 "tool"
					Content:    errMsg,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			g.Log().Debugf(ctx, "[工具 %d/%d] 参数: %v", idx+1, len(response.ToolCalls), args)

			// 调用工具
			result, mcpResult, err := tc.callSingleTool(ctx, serviceName, toolName, args, convID)
			if err != nil {
				errMsg := fmt.Sprintf("工具调用失败: %v", err)
				g.Log().Errorf(ctx, "[工具 %d/%d] %s", idx+1, len(response.ToolCalls), errMsg)

				// 添加错误响应到消息历史
				messages = append(messages, &schema.Message{
					Role:       schema.Tool, // 注意：这里应该是 Tool 而不是 "tool"
					Content:    errMsg,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			g.Log().Debugf(ctx, "[工具 %d/%d] 执行成功，结果长度: %d",
				idx+1, len(response.ToolCalls), len(result.Content))

			// 收集结果
			if result != nil {
				allDocuments = append(allDocuments, result)
			}
			if mcpResult != nil {
				allMCPResults = append(allMCPResults, mcpResult)
			}

			// 记录工具调用日志
			toolCallLog := map[string]interface{}{
				"service_name": serviceName,
				"tool_name":    toolName,
				"arguments":    args,
				"result":       mcpResult.Content,
			}
			toolCallLogs = append(toolCallLogs, toolCallLog)

			// 【关键】将工具执行结果添加到消息历史，供 LLM 下次调用时使用
			toolResultMsg := &schema.Message{
				Role:       schema.Tool, // 注意：这里应该是 Tool 而不是 "tool"
				Content:    mcpResult.Content,
				ToolCallID: toolCall.ID,
			}
			messages = append(messages, toolResultMsg)

			g.Log().Debugf(ctx, "[工具 %d/%d] 结果已添加到消息历史", idx+1, len(response.ToolCalls))
		}

		// 如果这是最后一次迭代，需要再调用一次 LLM 让它基于工具结果给出最终答案
		if iteration == maxIterations-1 {
			g.Log().Warning(ctx, "达到最大工具调用迭代次数，尝试获取最终答案")

			// 最后一次调用 LLM，不再提供工具（强制它给出最终答案）
			finalResponse, err := chatInstance.GenerateWithToolsAndSave(ctx, messages, nil)
			if err != nil {
				g.Log().Errorf(ctx, "获取最终答案失败: %v", err)
			} else {
				finalAnswer = finalResponse.Content
				// 注意：GenerateWithToolsAndSave已经保存了消息，这里不需要再保存
				g.Log().Debugf(ctx, "获取到最终答案（长度: %d）", len(finalAnswer))
			}
			break
		}
	}

	// 6. 返回结果
	return allDocuments, allMCPResults, nil
}
