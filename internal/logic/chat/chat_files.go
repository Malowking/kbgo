package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/formatter"
	"github.com/Malowking/kbgo/core/indexer"
	coreModel "github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/history"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/sashabaranov/go-openai"
)

// GetAnswerWithParsedFiles 使用已解析的文件内容进行多模态对话
func (x *Chat) GetAnswerWithParsedFiles(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, multimodalFiles []*common.MultimodalFile, fileContent string, fileImages []string, jsonFormat bool) (answer string, reasoningContent string, err error) {
	// 获取模型配置
	mc := coreModel.Registry.Get(modelID)
	if mc == nil {
		return "", "", fmt.Errorf("model not found: %s", modelID)
	}

	// 根据模型类型选择格式适配器
	var msgFormatter formatter.MessageFormatter
	if IsQwenModel(mc.Name) {
		msgFormatter = formatter.NewQwenFormatter()
	} else {
		msgFormatter = formatter.NewOpenAIFormatter()
	}

	// 创建模型服务
	modelService := coreModel.NewModelService(mc.APIKey, mc.BaseURL, msgFormatter)

	// 获取聊天历史
	chatHistory, err := x.eh.GetHistory(convID, 100)
	if err != nil {
		return "", "", err
	}

	// 构建多模态消息（只包含用户问题和多模态文件）
	userMessage, err := buildMultimodalMessageWithImages(ctx, question, multimodalFiles, fileImages, mc.Type)
	if err != nil {
		return "", "", fmt.Errorf("构建多模态消息失败: %w", err)
	}

	// 保存用户消息
	err = x.eh.SaveMessage(userMessage, convID)
	if err != nil {
		return "", "", err
	}

	// 构建system提示词
	systemPrompt := buildSystemPrompt(mc.Type, docs, fileContent, fileImages)

	// 构建消息列表
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
	}
	messages = append(messages, chatHistory...)
	messages = append(messages, userMessage)

	// 解析推理参数
	params := parseModelParams(mc.Extra)

	// 如果需要JSON格式化，设置ResponseFormat
	if jsonFormat {
		params.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	// 构建请求参数
	chatParams := coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         getFloat32OrDefault(params.Temperature, 0.7),
		MaxCompletionTokens: getIntOrDefault(params.MaxCompletionTokens, 2000),
		TopP:                getFloat32OrDefault(params.TopP, 0.9),
		FrequencyPenalty:    getFloat32OrDefault(params.FrequencyPenalty, 0.0),
		PresencePenalty:     getFloat32OrDefault(params.PresencePenalty, 0.0),
		N:                   getIntOrDefault(params.N, 1),
		Tools:               params.Tools,
		ToolChoice:          params.ToolChoice,
		ResponseFormat:      params.ResponseFormat,
	}

	// 记录开始时间
	start := time.Now()

	// 调用模型服务
	resp, err := modelService.ChatCompletion(ctx, chatParams)
	if err != nil {
		return "", "", fmt.Errorf("API调用失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", "", fmt.Errorf("received empty choices from API")
	}

	answerContent := resp.Choices[0].Message.Content
	thinkContent := resp.Choices[0].Message.ReasoningContent

	// 计算延迟
	latencyMs := time.Since(start).Milliseconds()

	// 创建assistant消息
	assistantMsg := &schema.Message{
		Role:    schema.Assistant,
		Content: answerContent,
	}

	// 只有当思考内容不为空时才添加到消息中
	if thinkContent != "" {
		assistantMsg.ReasoningContent = thinkContent
	}

	// 创建带指标的消息
	msgWithMetrics := &history.MessageWithMetrics{
		Message:    assistantMsg,
		LatencyMs:  int(latencyMs),
		TokensUsed: resp.Usage.TotalTokens,
	}

	err = x.eh.SaveMessageWithMetrics(msgWithMetrics, convID)
	if err != nil {
		g.Log().Error(ctx, "save assistant message err: %v", err)
		return "", "", err
	}

	return answerContent, thinkContent, nil
}

// GetAnswerWithFiles 统一的多模态对话处理（使用新架构）
func (x *Chat) GetAnswerWithFiles(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, files []*common.MultimodalFile) (answer string, reasoningContent string, err error) {
	// 获取模型配置
	mc := coreModel.Registry.Get(modelID)
	if mc == nil {
		return "", "", fmt.Errorf("model not found: %s", modelID)
	}

	// 根据模型类型选择格式适配器
	var msgFormatter formatter.MessageFormatter
	if IsQwenModel(mc.Name) {
		msgFormatter = formatter.NewQwenFormatter()
	} else {
		msgFormatter = formatter.NewOpenAIFormatter()
	}

	// 创建模型服务
	modelService := coreModel.NewModelService(mc.APIKey, mc.BaseURL, msgFormatter)

	// 获取聊天历史
	chatHistory, err := x.eh.GetHistory(convID, 100)
	if err != nil {
		return "", "", err
	}

	// 分离多模态文件和文档文件
	multimodalFiles, documentFiles := separateFilesByType(files)

	// 检查会话中是否已有文档信息（用于多轮对话）
	existingFileContent, existingFileImages, err := getConversationDocumentInfo(ctx, x.eh, convID)
	if err != nil {
		g.Log().Warningf(ctx, "Failed to get existing document info: %v", err)
	}

	var fileContent string
	var fileImages []string

	// 如果本次有新的文档文件上传，解析它们
	if len(documentFiles) > 0 {
		fileContent, fileImages, err = parseDocumentFiles(ctx, documentFiles)
		if err != nil {
			g.Log().Warningf(ctx, "Failed to parse document files: %v", err)
			fileContent = ""
		}

		// 保存文档信息到会话metadata（仅第一次）
		if existingFileContent == "" {
			err = saveConversationDocumentInfo(ctx, x.eh, convID, documentFiles, fileContent, fileImages)
			if err != nil {
				g.Log().Errorf(ctx, "Failed to save document info to conversation: %v", err)
			}
		}
	} else if existingFileContent != "" {
		// 多轮对话，使用已保存的文档信息
		fileContent = existingFileContent
		fileImages = existingFileImages
		g.Log().Infof(ctx, "Reusing existing document info from conversation metadata")
	}

	// 构建多模态消息（只包含用户问题和多模态文件）
	userMessage, err := buildMultimodalMessageWithImages(ctx, question, multimodalFiles, fileImages, mc.Type)
	if err != nil {
		return "", "", fmt.Errorf("构建多模态消息失败: %w", err)
	}

	// 保存用户消息
	err = x.eh.SaveMessage(userMessage, convID)
	if err != nil {
		return "", "", err
	}

	// 构建system提示词
	systemPrompt := buildSystemPrompt(mc.Type, docs, fileContent, fileImages)

	// 构建消息列表
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
	}
	messages = append(messages, chatHistory...)
	messages = append(messages, userMessage)

	// 解析推理参数
	params := parseModelParams(mc.Extra)

	// 构建请求参数
	chatParams := coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         getFloat32OrDefault(params.Temperature, 0.7),
		MaxCompletionTokens: getIntOrDefault(params.MaxCompletionTokens, 2000),
		TopP:                getFloat32OrDefault(params.TopP, 0.9),
		FrequencyPenalty:    getFloat32OrDefault(params.FrequencyPenalty, 0.0),
		PresencePenalty:     getFloat32OrDefault(params.PresencePenalty, 0.0),
		N:                   getIntOrDefault(params.N, 1),
		Tools:               params.Tools,
		ToolChoice:          params.ToolChoice,
		ResponseFormat:      params.ResponseFormat,
	}

	// 记录开始时间
	start := time.Now()

	// 调用模型服务
	resp, err := modelService.ChatCompletion(ctx, chatParams)
	if err != nil {
		return "", "", fmt.Errorf("API调用失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", "", fmt.Errorf("received empty choices from API")
	}

	answerContent := resp.Choices[0].Message.Content
	thinkContent := resp.Choices[0].Message.ReasoningContent

	// 计算延迟
	latencyMs := time.Since(start).Milliseconds()

	// 创建assistant消息
	assistantMsg := &schema.Message{
		Role:    schema.Assistant,
		Content: answerContent,
	}

	// 只有当思考内容不为空时才添加到消息中
	if thinkContent != "" {
		assistantMsg.ReasoningContent = thinkContent
	}

	// 创建带指标的消息
	msgWithMetrics := &history.MessageWithMetrics{
		Message:    assistantMsg,
		LatencyMs:  int(latencyMs),
		TokensUsed: resp.Usage.TotalTokens,
	}

	err = x.eh.SaveMessageWithMetrics(msgWithMetrics, convID)
	if err != nil {
		g.Log().Error(ctx, "save assistant message err: %v", err)
		return "", "", err
	}

	return answerContent, thinkContent, nil
}

// GetAnswerStreamWithFiles 统一的多模态流式对话处理
func (x *Chat) GetAnswerStreamWithFiles(ctx context.Context, modelID string, convID string, docs []*schema.Document, question string, files []*common.MultimodalFile, jsonFormat bool) (answer *schema.StreamReader[*schema.Message], err error) {
	// 获取模型配置
	mc := coreModel.Registry.Get(modelID)
	if mc == nil {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	// 根据模型类型选择格式适配器
	var msgFormatter formatter.MessageFormatter
	if IsQwenModel(mc.Name) {
		msgFormatter = formatter.NewQwenFormatter()
	} else {
		msgFormatter = formatter.NewOpenAIFormatter()
	}

	// 创建模型服务
	modelService := coreModel.NewModelService(mc.APIKey, mc.BaseURL, msgFormatter)

	// 获取聊天历史
	chatHistory, err := x.eh.GetHistory(convID, 100)
	if err != nil {
		return nil, err
	}

	// 分离多模态文件和文档文件
	multimodalFiles, documentFiles := separateFilesByType(files)

	// 检查会话中是否已有文档信息（用于多轮对话）
	existingFileContent, existingFileImages, err := getConversationDocumentInfo(ctx, x.eh, convID)
	if err != nil {
		g.Log().Warningf(ctx, "Failed to get existing document info: %v", err)
	}

	var fileContent string
	var fileImages []string

	// 如果本次有新的文档文件上传，解析它们
	if len(documentFiles) > 0 {
		fileContent, fileImages, err = parseDocumentFiles(ctx, documentFiles)
		if err != nil {
			g.Log().Warningf(ctx, "Failed to parse document files: %v", err)
			fileContent = ""
		}

		// 保存文档信息到会话metadata（仅第一次）
		if existingFileContent == "" {
			err = saveConversationDocumentInfo(ctx, x.eh, convID, documentFiles, fileContent, fileImages)
			if err != nil {
				g.Log().Errorf(ctx, "Failed to save document info to conversation: %v", err)
			}
		}
	} else if existingFileContent != "" {
		// 多轮对话，使用已保存的文档信息
		fileContent = existingFileContent
		fileImages = existingFileImages
		g.Log().Infof(ctx, "Reusing existing document info from conversation metadata")
	}

	// 构建多模态消息（只包含用户问题和多模态文件）
	userMessage, err := buildMultimodalMessageWithImages(ctx, question, multimodalFiles, fileImages, mc.Type)
	if err != nil {
		return nil, fmt.Errorf("构建多模态消息失败: %w", err)
	}

	// 保存用户消息
	err = x.eh.SaveMessage(userMessage, convID)
	if err != nil {
		return nil, err
	}

	// 构建system提示词
	systemPrompt := buildSystemPrompt(mc.Type, docs, fileContent, fileImages)

	// 构建消息列表
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
	}
	messages = append(messages, chatHistory...)
	messages = append(messages, userMessage)

	// 解析推理参数
	params := parseModelParams(mc.Extra)

	// 如果需要JSON格式化，设置ResponseFormat
	if jsonFormat {
		params.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	// 构建请求参数
	chatParams := coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         getFloat32OrDefault(params.Temperature, 0.7),
		MaxCompletionTokens: getIntOrDefault(params.MaxCompletionTokens, 2000),
		TopP:                getFloat32OrDefault(params.TopP, 0.9),
		FrequencyPenalty:    getFloat32OrDefault(params.FrequencyPenalty, 0.0),
		PresencePenalty:     getFloat32OrDefault(params.PresencePenalty, 0.0),
		N:                   getIntOrDefault(params.N, 1),
		Tools:               params.Tools,
		ToolChoice:          params.ToolChoice,
		ResponseFormat:      params.ResponseFormat,
	}

	// 记录开始时间
	start := time.Now()

	// 调用模型服务流式接口
	stream, err := modelService.ChatCompletionStream(ctx, chatParams)
	if err != nil {
		return nil, fmt.Errorf("API调用失败: %w", err)
	}

	// 创建 Pipe 用于流式传输
	streamReader, streamWriter := schema.Pipe[*schema.Message](10)

	// 保留原始 context 用于取消控制
	originalCtx := ctx
	// 使用 Background context 避免父 context 取消影响流式处理的完整性
	ctx = context.Background()

	// 启动goroutine处理流式响应
	go func() {
		defer streamWriter.Close()
		defer stream.Close()

		var fullContent strings.Builder
		var fullReasoningContent strings.Builder
		var tokenCount int

		for {
			// 检查客户端是否断开连接
			select {
			case <-originalCtx.Done():
				g.Log().Warning(ctx, "Stream cancelled by client, stopping goroutine")
				return
			default:
			}

			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// 流结束，保存完整消息
				assistantMsg := &schema.Message{
					Role:    schema.Assistant,
					Content: fullContent.String(),
				}

				// 只有当思考内容不为空时才添加到消息中
				if fullReasoningContent.Len() > 0 {
					assistantMsg.ReasoningContent = fullReasoningContent.String()
				}

				// 计算延迟
				latencyMs := time.Since(start).Milliseconds()

				// 创建带指标的消息
				msgWithMetrics := &history.MessageWithMetrics{
					Message:    assistantMsg,
					LatencyMs:  int(latencyMs),
					TokensUsed: tokenCount,
				}

				// 异步保存消息
				saveErr := x.eh.SaveMessageWithMetrics(msgWithMetrics, convID)
				if saveErr != nil {
					g.Log().Errorf(ctx, "save assistant message err: %v", saveErr)
				}

				return
			}

			if err != nil {
				g.Log().Errorf(ctx, "stream receive error: %v", err)
				streamWriter.Send(&schema.Message{
					Role:    schema.Assistant,
					Content: "",
				}, err)
				return
			}

			// 处理流式响应
			if len(response.Choices) > 0 {
				delta := response.Choices[0].Delta.Content
				rdelta := response.Choices[0].Delta.ReasoningContent

				// 创建增量消息
				chunk := &schema.Message{
					Role: schema.Assistant,
				}

				// 处理普通内容
				if delta != "" {
					fullContent.WriteString(delta)
					chunk.Content = delta
				}

				// 处理思考内容
				if rdelta != "" {
					fullReasoningContent.WriteString(rdelta)
					chunk.ReasoningContent = rdelta
				}

				// 只有当有内容或思考内容时才发送
				if delta != "" || rdelta != "" {
					closed := streamWriter.Send(chunk, nil)
					if closed {
						g.Log().Warningf(ctx, "stream writer closed unexpectedly")
						return
					}
				}

				// 累计token数量（如果有usage信息）
				if response.Usage != nil {
					tokenCount = response.Usage.TotalTokens
				}
			}
		}
	}()

	return streamReader, nil
}

// formatDocumentsForQwen 格式化文档为可读的字符串
func formatDocumentsForQwen(docs []*schema.Document) string {
	if len(docs) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("参考资料:\n")
	for i, doc := range docs {
		builder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, doc.Content))
	}
	return builder.String()
}

// IsQwenModel 判断是否为Qwen模型
func IsQwenModel(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), "qwen")
}

// buildMultimodalMessageWithImages 构建多模态消息，支持从历史对话中提取文档图片
func buildMultimodalMessageWithImages(ctx context.Context, text string, files []*common.MultimodalFile, fileImages []string, modelType coreModel.ModelType) (*schema.Message, error) {
	var userInputParts []schema.MessageInputPart

	// 添加文本部分
	if text != "" {
		userInputParts = append(userInputParts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: text,
		})
	}

	// 添加用户上传的多模态文件部分
	for _, file := range files {
		part, err := buildFilePart(file)
		if err != nil {
			g.Log().Errorf(ctx, "Failed to build file part for %s: %v", file.FileName, err)
			continue
		}
		userInputParts = append(userInputParts, part)
	}

	// 如果是多模态模型且有文档图片，读取并添加图片
	if modelType == coreModel.ModelTypeMultimodal && len(fileImages) > 0 {
		for _, imgURL := range fileImages {
			base64Data, mimeType, err := downloadImageFromURL(ctx, imgURL)
			if err != nil {
				g.Log().Warningf(ctx, "Failed to download image %s: %v", imgURL, err)
				continue
			}

			imagePart := schema.MessageInputPart{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						URL:        &imgURL,
						Base64Data: &base64Data,
						MIMEType:   mimeType,
					},
				},
			}
			userInputParts = append(userInputParts, imagePart)
		}
	}

	// 如果没有任何内容，返回纯文本消息
	if len(userInputParts) == 0 {
		return &schema.Message{
			Role:    schema.User,
			Content: text,
		}, nil
	}

	return &schema.Message{
		Role:                  schema.User,
		UserInputMultiContent: userInputParts,
	}, nil
}

// buildFilePart 构建文件部分
func buildFilePart(file *common.MultimodalFile) (schema.MessageInputPart, error) {
	switch file.FileType {
	case common.FileTypeImage:
		// 读取图片文件
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			return schema.MessageInputPart{}, fmt.Errorf("failed to read image file: %w", err)
		}

		// 获取MIME类型
		ext := filepath.Ext(file.FileName)
		mimeType := getMimeTypeForFile(ext)

		// 编码为base64
		base64Data := base64.StdEncoding.EncodeToString(data)

		// 使用MessageInputPart，存储文件路径到URL，base64用于API调用
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL:        &file.FilePath, // 存储文件路径
					Base64Data: &base64Data,    // base64数据用于API调用
					MIMEType:   mimeType,
				},
			},
		}, nil

	default:
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: fmt.Sprintf("[文件: %s]", file.FileName),
		}, nil
	}
}

// getMimeTypeForFile 获取MIME类型
// 已废弃：使用 common.GetMimeType 替代
func getMimeTypeForFile(ext string) string {
	return common.GetMimeType(ext)
}

// 辅助函数
func getFloat32OrDefault(val *float32, defaultVal float32) float32 {
	if val != nil {
		return *val
	}
	return defaultVal
}

func getIntOrDefault(val *int, defaultVal int) int {
	if val != nil {
		return *val
	}
	return defaultVal
}

// separateFilesByType 将文件分离为多模态文件和文档文件
func separateFilesByType(files []*common.MultimodalFile) (multimodalFiles []*common.MultimodalFile, documentFiles []*common.MultimodalFile) {
	for _, file := range files {
		if file.FileType == common.FileTypeImage ||
			file.FileType == common.FileTypeAudio ||
			file.FileType == common.FileTypeVideo {
			multimodalFiles = append(multimodalFiles, file)
		} else {
			documentFiles = append(documentFiles, file)
		}
	}
	return
}

// ParseDocumentFiles 解析文档文件，调用Python服务获取全文和图片（公开函数）
func ParseDocumentFiles(ctx context.Context, files []*common.MultimodalFile) (string, []string, error) {
	return parseDocumentFiles(ctx, files)
}

// parseDocumentFiles 解析文档文件，调用Python服务获取全文和图片（内部函数）
func parseDocumentFiles(ctx context.Context, files []*common.MultimodalFile) (string, []string, error) {
	if len(files) == 0 {
		return "", nil, nil
	}

	var allContent strings.Builder
	var allImages []string

	// 创建文件解析加载器，chunk_size=-1表示不切分，imageURLFormat=false表示返回相对路径
	loader, err := indexer.NewFileParseLoaderForChat(ctx, -1, 0, "")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create file parse loader: %w", err)
	}

	// 获取项目根目录（用于拼接图片路径）
	projectRoot, err := os.Getwd()
	if err != nil {
		g.Log().Warningf(ctx, "Failed to get working directory: %v", err)
		projectRoot = ""
	}

	for _, file := range files {
		g.Log().Infof(ctx, "Parsing document file: %s (type: %s)", file.FileName, file.FileType)

		// 调用Python服务解析文件，chunk_size=-1表示不切分，imageURLFormat=false返回相对路径
		docs, err := loader.Load(ctx, file.FilePath)
		if err != nil {
			g.Log().Errorf(ctx, "Failed to parse file %s: %v", file.FileName, err)
			continue
		}

		// 提取文本内容
		for _, doc := range docs {
			allContent.WriteString(doc.Content)
			allContent.WriteString("\n\n")

			// 从metadata中提取图片URLs（只在第一个document中有）
			if imageURLs, ok := doc.MetaData["image_urls"].([]interface{}); ok {
				for _, imgURL := range imageURLs {
					if imgStr, ok := imgURL.(string); ok {
						// 如果返回的是相对路径（从image/开始），需要拼接为绝对路径
						if strings.HasPrefix(imgStr, "image/") && projectRoot != "" {
							// 拼接完整路径：projectRoot/upload/image/xxx.png
							// 从 "image/xxx.png" 提取 "xxx.png" 部分
							imageName := strings.TrimPrefix(imgStr, "image/")
							fullPath := filepath.Join(projectRoot, "upload", "image", imageName)
							allImages = append(allImages, fullPath)
							g.Log().Infof(ctx, "Converted relative image path '%s' to absolute path '%s'", imgStr, fullPath)
						} else {
							// 已经是绝对路径，直接使用
							allImages = append(allImages, imgStr)
						}
					}
				}
			} else if imageURLs, ok := doc.MetaData["image_urls"].([]string); ok {
				// 兼容直接是字符串数组的情况
				for _, imgStr := range imageURLs {
					// 如果返回的是相对路径（从image/开始），需要拼接为绝对路径
					if strings.HasPrefix(imgStr, "image/") && projectRoot != "" {
						// 拼接完整路径：projectRoot/upload/image/xxx.png
						imageName := strings.TrimPrefix(imgStr, "image/")
						fullPath := filepath.Join(projectRoot, "upload", "image", imageName)
						allImages = append(allImages, fullPath)
						g.Log().Infof(ctx, "Converted relative image path '%s' to absolute path '%s'", imgStr, fullPath)
					} else {
						// 已经是绝对路径，直接使用
						allImages = append(allImages, imgStr)
					}
				}
			}
		}
	}

	return allContent.String(), allImages, nil
}

// buildSystemPrompt 根据模型类型构建system提示词
func buildSystemPrompt(modelType coreModel.ModelType, docs []*schema.Document, fileContent string, imageURLs []string) string {
	var builder strings.Builder

	// 基础提示词
	builder.WriteString("你是一个专业的AI助手，能够根据提供的参考信息准确回答用户问题。\n")

	// 如果有检索到的文档
	if len(docs) > 0 {
		builder.WriteString("\n参考资料:\n")
		for i, doc := range docs {
			builder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, doc.Content))
		}
	}

	// 如果有文件内容，移除其中的图片占位符（因为图片已通过user消息传入）
	if fileContent != "" {
		// 移除图片占位符Markdown语法
		cleanedContent := removeImagePlaceholders(fileContent)
		builder.WriteString("\n文档内容:\n")
		builder.WriteString(cleanedContent)
		builder.WriteString("\n")
	}

	// 根据模型类型添加图片相关提示
	if len(imageURLs) > 0 {
		if modelType == coreModel.ModelTypeMultimodal {
			// 多模态模型：提醒有图片需要解析
			builder.WriteString(fmt.Sprintf("\n注意：该文档包含 %d 张图片，这些图片已按照文档中出现的顺序传入用户消息的图片部分。请结合图片内容进行回答。\n", len(imageURLs)))
			builder.WriteString("重要提示：在回答问题时，请直接引用和描述图片内容，不要提及任何图片路径、文件路径或占位符信息。用户看不到这些技术细节，只需要你对图片内容的理解和描述。\n")
		} else {
			// 普通LLM：说明有图片但无法解析
			builder.WriteString(fmt.Sprintf("\n注意：该文档包含 %d 张图片，但当前模型无法解析图片内容。请基于文本内容回答。\n", len(imageURLs)))
			builder.WriteString("重要提示：文档中可能包含图片占位符（如路径信息），这些只是技术标记，不要在回答中提及这些路径或占位符。\n")
		}
	}

	// 如果没有任何参考信息
	if len(docs) == 0 && fileContent == "" {
		builder.WriteString("如果没有提供参考信息，请根据你的知识自由回答用户问题。\n")
	}

	return builder.String()
}

// getConversationDocumentInfo 从会话metadata中获取文档信息
func getConversationDocumentInfo(ctx context.Context, eh *history.Manager, convID string) (string, []string, error) {
	// 获取会话信息
	conv, err := getConversation(convID)
	if err != nil {
		// 只在真正的错误时返回错误，不在会话不存在时返回错误
		return "", nil, err
	}

	// 会话不存在或没有metadata，返回空值（这是正常情况，不是错误）
	if conv == nil || len(conv.Metadata) == 0 {
		return "", nil, nil
	}

	var metadata map[string]interface{}
	err = json.Unmarshal(conv.Metadata, &metadata)
	if err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal conversation metadata: %w", err)
	}

	fileContent, _ := metadata["file_content"].(string)

	var fileImages []string
	if imgs, ok := metadata["file_images"].([]interface{}); ok {
		for _, img := range imgs {
			if imgStr, ok := img.(string); ok {
				fileImages = append(fileImages, imgStr)
			}
		}
	}

	return fileContent, fileImages, nil
}

// saveConversationDocumentInfo 保存文档信息到会话metadata
func saveConversationDocumentInfo(ctx context.Context, eh *history.Manager, convID string, files []*common.MultimodalFile, fileContent string, fileImages []string) error {
	conv, err := getConversation(convID)
	if err != nil {
		return err
	}

	// 会话不存在，静默忽略（可能是首次创建会话，conversation还未创建）
	if conv == nil {
		g.Log().Infof(ctx, "Conversation %s not found yet, skipping metadata save (will be saved on next message)", convID)
		return nil
	}

	// 解析现有metadata
	var metadata map[string]interface{}
	if len(conv.Metadata) > 0 {
		err = json.Unmarshal(conv.Metadata, &metadata)
		if err != nil {
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}

	// 保存文档路径
	docPaths := make([]string, len(files))
	for i, file := range files {
		docPaths[i] = file.FilePath
	}
	metadata["document_files"] = docPaths
	metadata["file_content"] = fileContent
	metadata["file_images"] = fileImages

	// 序列化metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// 更新conversation
	return updateConversationMetadata(convID, metadataJSON)
}

// getConversation 获取会话信息
func getConversation(convID string) (*gormModel.Conversation, error) {
	return dao.Conversation.GetByConvID(nil, convID)
}

// updateConversationMetadata 更新会话metadata
func updateConversationMetadata(convID string, metadata gormModel.JSON) error {
	return dao.Conversation.UpdateMetadata(nil, convID, metadata)
}

// downloadImageFromURL 从URL或本地路径读取图片并返回base64编码的数据
func downloadImageFromURL(ctx context.Context, imageURL string) (string, string, error) {
	// 判断是否为本地文件路径（绝对路径）
	if filepath.IsAbs(imageURL) {
		// 读取本地文件
		g.Log().Infof(ctx, "Reading local image file: %s", imageURL)
		data, err := os.ReadFile(imageURL)
		if err != nil {
			return "", "", fmt.Errorf("failed to read local image file: %w", err)
		}

		// 编码为base64
		base64Data := base64.StdEncoding.EncodeToString(data)

		// 从文件路径获取扩展名并确定MIME类型
		ext := filepath.Ext(imageURL)
		mimeType := getMimeTypeForFile(ext)

		g.Log().Infof(ctx, "Successfully read local image: %s, size: %d bytes, mime: %s", imageURL, len(data), mimeType)
		return base64Data, mimeType, nil
	}

	// HTTP URL：发送HTTP GET请求
	g.Log().Infof(ctx, "Downloading image from URL: %s", imageURL)
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to download image: status code %d", resp.StatusCode)
	}

	// 读取图片数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read image data: %w", err)
	}

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// 从URL获取文件扩展名并确定MIME类型
	ext := filepath.Ext(imageURL)
	mimeType := getMimeTypeForFile(ext)

	g.Log().Infof(ctx, "Successfully downloaded image: %s, size: %d bytes, mime: %s", imageURL, len(data), mimeType)
	return base64Data, mimeType, nil
}

// removeImagePlaceholders 移除文本中的图片占位符
// 匹配格式: ![image-0](http://127.0.0.1:8002/images/xxx.png)
func removeImagePlaceholders(text string) string {
	// 使用strings.Replace移除所有图片占位符
	// 由于图片占位符格式是 ![image-N](...), 我们需要逐行处理
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		// 如果这一行只包含图片占位符，跳过
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "![image-") && strings.Contains(trimmed, "](http") {
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
