package rag

import (
	"context"
	"encoding/json"
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
		mcpDocs, err := c.callMCPTools(ctx, req)
		if err != nil {
			g.Log().Errorf(ctx, "MCP工具调用失败: %v", err)
		} else {
			// 将MCP结果合并到上下文中
			documents = append(documents, mcpDocs...)
			// 提取MCP结果用于返回
			mcpResults = c.extractMCPResults(mcpDocs)
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

	return res, nil
}

// handleStreamChat 处理流式聊天请求
func (c *ControllerV1) handleStreamChat(ctx context.Context, req *v1.ChatReq) error {
	var streamReader *schema.StreamReader[*schema.Message]

	// 获取检索配置
	cfg := rag2.GetRetrieverConfig()

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
			g.Log().Error(ctx, err)
			return err
		}
		documents = retriever.Document
	}

	var mcpResults []*v1.MCPResult
	// 如果启用MCP，则执行MCP逻辑
	if req.UseMCP {
		mcpDocs, err := c.callMCPTools(ctx, req)
		if err != nil {
			g.Log().Errorf(ctx, "MCP工具调用失败: %v", err)
		} else {
			// 将MCP结果合并到上下文中
			documents = append(documents, mcpDocs...)
			// 提取MCP结果用于返回
			mcpResults = c.extractMCPResults(mcpDocs)
		}
	}

	// 获取Chat实例
	chatI := chat.GetChat()
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

			// 调用工具
			result, err := mcpClient.CallTool(ctx, tool.Name, map[string]interface{}{
				"question": req.Question,
			})

			// 计算耗时
			duration := int(time.Since(startTime).Milliseconds())

			// 序列化请求和响应
			reqPayload, _ := json.Marshal(map[string]interface{}{
				"question": req.Question,
			})
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
