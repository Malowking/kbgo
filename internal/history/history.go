package history

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/os/gctx"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageWithMetrics 带指标的消息结构
type MessageWithMetrics struct {
	*schema.Message
	TokensUsed int
	LatencyMs  int
	TraceID    string
	ToolCalls  []*schema.ToolCall
}

// Manager 聊天历史管理器
type Manager struct {
	db *gorm.DB
}

// NewManager 创建新的聊天历史管理器
func NewManager() *Manager {
	return &Manager{
		db: dao.GetDB(),
	}
}

// SaveMessageWithMetrics 保存带指标的消息
func (h *Manager) SaveMessageWithMetrics(message *MessageWithMetrics, convID string) error {
	// 使用全局异步保存器
	asyncSaver := GetGlobalAsyncSaver()

	// 异步保存，不等待结果
	asyncSaver.SaveMessageAsync(message, convID)

	return nil
}

// SaveMessage 异步保存消息，支持自定义时间戳
func (h *Manager) SaveMessage(message *schema.Message, convID string, metadata map[string]interface{}, createTime *time.Time) error {
	// 使用全局异步保存器
	asyncSaver := GetGlobalAsyncSaver()

	// 如果没有提供时间戳，使用当前时间
	if createTime == nil {
		now := time.Now()
		createTime = &now
	}

	// 构建保存任务
	task := &SaveMetadataTask{
		Message:    message,
		ConvID:     convID,
		Metadata:   metadata,
		CreateTime: createTime,
		Result:     nil, // 不等待结果
	}

	// 异步保存
	asyncSaver.SaveMessageWithMetadataAsync(task)

	return nil
}

// SaveMessageWithMetadataSync 同步保存带元数据的消息
func (h *Manager) SaveMessageWithMetadataSync(message *schema.Message, convID string, metadata map[string]interface{}, createTime *time.Time) error {
	// 确保对话存在
	if err := h.ensureConversationExists(convID); err != nil {
		return err
	}

	// 如果没有提供时间戳，使用当前时间
	if createTime == nil {
		now := time.Now()
		createTime = &now
	}

	// 如果是 tool role 的消息，将其附加到对应的 assistant 消息中
	if message.Role == schema.Tool && message.ToolCallID != "" {
		return h.attachToolResultToAssistantMessage(convID, message, metadata, createTime)
	}

	// 处理元数据
	var metadataJSON gormModel.JSON
	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err != nil {
			return errors.Newf(errors.ErrInternalError, "failed to marshal metadata: %v", err)
		}
		metadataJSON = gormModel.JSON(data)
	}

	// 处理工具调用
	var toolCallsJSON gormModel.JSON
	if message.ToolCalls != nil && len(message.ToolCalls) > 0 {
		data, err := json.Marshal(message.ToolCalls)
		if err != nil {
			return errors.Newf(errors.ErrInternalError, "failed to marshal tool calls: %v", err)
		}
		toolCallsJSON = gormModel.JSON(data)
	}

	// 创建消息记录
	msg := &gormModel.Message{
		MsgID:      generateMessageID(),
		ConvID:     convID,
		Role:       string(message.Role),
		CreateTime: createTime,
		Metadata:   metadataJSON,
		ToolCalls:  toolCallsJSON,
	}

	// 处理内容块 - 支持多模态内容
	var contents []*gormModel.MessageContent

	// 优先处理 UserInputMultiContent（新版多模态字段）
	if len(message.UserInputMultiContent) > 0 {
		for i, part := range message.UserInputMultiContent {
			content := &gormModel.MessageContent{
				SortOrder:  i,
				CreateTime: createTime,
			}

			switch part.Type {
			case schema.MessagePartTypeText:
				content.ContentType = "text"
				content.TextContent = part.Text

			case schema.MessagePartTypeImageURL:
				if part.Image != nil {
					content.ContentType = "image_url"
					// 存储文件路径到media_url
					if part.Image.URL != nil {
						content.MediaURL = *part.Image.URL
					}
				}

			case schema.MessagePartTypeAudioURL:
				content.ContentType = "audio_url"
				if part.Audio != nil && part.Audio.URL != nil {
					content.MediaURL = *part.Audio.URL
				}

			case schema.MessagePartTypeVideoURL:
				content.ContentType = "video_url"
				if part.Video != nil && part.Video.URL != nil {
					content.MediaURL = *part.Video.URL
				}
			}

			contents = append(contents, content)
		}
	} else if message.Content != "" {
		// 普通文本消息
		content := &gormModel.MessageContent{
			ContentType: "text",
			TextContent: message.Content,
			SortOrder:   0,
			CreateTime:  createTime,
		}
		contents = append(contents, content)
	}

	// 如果没有任何内容，至少保存一个空文本内容
	if len(contents) == 0 {
		content := &gormModel.MessageContent{
			ContentType: "text",
			TextContent: "",
			SortOrder:   0,
			CreateTime:  createTime,
		}
		contents = append(contents, content)
	}

	// 直接写数据库
	return dao.Message.CreateWithContents(nil, msg, contents)
}

// attachToolResultToAssistantMessage 将工具调用结果附加到对应的 assistant 消息中
func (h *Manager) attachToolResultToAssistantMessage(convID string, toolMessage *schema.Message, metadata map[string]interface{}, now *time.Time) error {
	// 1. 查找包含该 tool_call_id 的 assistant 消息
	var messages []gormModel.Message
	err := h.db.Where("conv_id = ? AND role = ? AND tool_calls IS NOT NULL", convID, "assistant").
		Order("create_time DESC"). // 按时间倒序
		Find(&messages).Error

	if err != nil {
		return errors.Newf(errors.ErrDatabaseQuery, "查询 assistant 消息失败: %v", err)
	}

	// 2. 遍历消息，找到包含该 tool_call_id 的消息
	var assistantMsg *gormModel.Message
	targetToolCallID := toolMessage.ToolCallID

	for i := range messages {
		if len(messages[i].ToolCalls) > 0 {
			var toolCalls []schema.ToolCall
			if err := json.Unmarshal(messages[i].ToolCalls, &toolCalls); err == nil {
				// 检查是否包含目标 tool_call_id
				for _, tc := range toolCalls {
					if tc.ID == targetToolCallID {
						assistantMsg = &messages[i]
						break
					}
				}
				if assistantMsg != nil {
					break
				}
			}
		}
	}

	if assistantMsg == nil {
		g.Log().Warningf(gctx.New(), "未找到包含 tool_call_id=%s 的 assistant 消息", targetToolCallID)
		// 降级：仍然保存为独立消息（兼容旧逻辑）
		return h.saveToolMessageAsStandalone(toolMessage, convID, metadata, now)
	}

	// 3. 获取该 assistant 消息的最大 sort_order
	var maxSortOrder int
	err = h.db.Model(&gormModel.MessageContent{}).
		Where("msg_id = ?", assistantMsg.MsgID).
		Select("COALESCE(MAX(sort_order), -1)").
		Scan(&maxSortOrder).Error

	if err != nil {
		return errors.Newf(errors.ErrDatabaseQuery, "查询最大 sort_order 失败: %v", err)
	}

	// 4. 创建 tool 类型的 message_content
	toolMetadata := map[string]interface{}{
		"tool_call_id": toolMessage.ToolCallID,
	}

	// 添加工具名称和参数（如果有）
	if metadata != nil {
		if toolName, ok := metadata["tool_name"].(string); ok {
			toolMetadata["tool_name"] = toolName
		}
		if toolArgs, ok := metadata["tool_args"]; ok {
			toolMetadata["tool_args"] = toolArgs
		}
	}

	metadataJSON, err := json.Marshal(toolMetadata)
	if err != nil {
		return errors.Newf(errors.ErrInternalError, "序列化 metadata 失败: %v", err)
	}

	toolContent := &gormModel.MessageContent{
		MsgID:       assistantMsg.MsgID,
		ContentType: "tool",
		TextContent: toolMessage.Content,
		Metadata:    gormModel.JSON(metadataJSON),
		SortOrder:   maxSortOrder + 1,
		CreateTime:  now,
	}

	// 4. 保存 tool 到数据库
	if err := h.db.Create(toolContent).Error; err != nil {
		return errors.Newf(errors.ErrDatabaseInsert, "保存 tool 失败: %v", err)
	}
	return nil
}

// saveToolMessageAsStandalone 将 tool 消息保存为独立消息
func (h *Manager) saveToolMessageAsStandalone(message *schema.Message, convID string, metadata map[string]interface{}, now *time.Time) error {
	g.Log().Warningf(gctx.New(), "降级：将 tool 消息保存为独立消息")

	// 处理元数据
	var metadataJSON gormModel.JSON
	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err != nil {
			return errors.Newf(errors.ErrInternalError, "failed to marshal metadata: %v", err)
		}
		metadataJSON = data
	}

	// 创建消息记录
	msg := &gormModel.Message{
		MsgID:      generateMessageID(),
		ConvID:     convID,
		Role:       string(message.Role),
		CreateTime: now,
		Metadata:   metadataJSON,
	}

	// 创建内容块
	toolMetadata := map[string]interface{}{
		"tool_call_id": message.ToolCallID,
	}
	if metadata != nil {
		if toolName, ok := metadata["tool_name"].(string); ok {
			toolMetadata["tool_name"] = toolName
		}
		if toolArgs, ok := metadata["tool_args"]; ok {
			toolMetadata["tool_args"] = toolArgs
		}
	}
	contentMetadataJSON, _ := json.Marshal(toolMetadata)

	content := &gormModel.MessageContent{
		ContentType: "text",
		TextContent: message.Content,
		SortOrder:   0,
		CreateTime:  now,
		Metadata:    gormModel.JSON(contentMetadataJSON),
	}

	return dao.Message.CreateWithContents(nil, msg, []*gormModel.MessageContent{content})
}

// GetHistory 获取聊天历史
// 返回按msg_id组装好的消息结构，每个assistant消息可能包含多个content块
func (h *Manager) GetHistory(convID string, limit int) ([]*schema.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	// 获取消息列表
	messages, _, err := dao.Message.ListByConvID(nil, convID, 1, limit)
	if err != nil {
		return nil, err
	}

	// 获取所有消息ID
	var msgIDs []string
	for _, msg := range messages {
		msgIDs = append(msgIDs, msg.MsgID)
	}

	// 批量获取内容块
	contents, err := dao.MessageContent.ListByMsgIDs(nil, msgIDs)
	if err != nil {
		return nil, err
	}

	// 按消息ID组织内容块
	contentMap := make(map[string][]*gormModel.MessageContent)
	for _, content := range contents {
		contentMap[content.MsgID] = append(contentMap[content.MsgID], content)
	}

	// 转换为 schema.Message
	result := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		// 获取该消息的内容块
		msgContents := contentMap[msg.MsgID]

		schemaMsg := &schema.Message{
			Role:  schema.RoleType(msg.Role),
			Extra: make(map[string]any),
		}

		// 如果消息有 tool_calls，也需要加载
		if len(msg.ToolCalls) > 0 {
			var toolCalls []schema.ToolCall
			if err := json.Unmarshal(msg.ToolCalls, &toolCalls); err == nil && len(toolCalls) > 0 {
				schemaMsg.ToolCalls = toolCalls
			}
		}

		// 保存创建时间到Extra字段
		if msg.CreateTime != nil {
			schemaMsg.Extra["create_time"] = msg.CreateTime.Format(time.RFC3339)
		}

		// 保存msg_id到Extra字段，用于前端识别
		schemaMsg.Extra["msg_id"] = msg.MsgID

		// 分离 tool 和其他内容
		var normalContents []*gormModel.MessageContent
		var toolContents []*gormModel.MessageContent

		for _, content := range msgContents {
			if content.ContentType == "tool" {
				toolContents = append(toolContents, content)
			} else {
				normalContents = append(normalContents, content)
			}
		}

		// 如果有多个内容块或包含非文本内容，构建MultiContent
		if len(normalContents) > 1 || (len(normalContents) == 1 && normalContents[0].ContentType != "text") {
			var multiContent []schema.MessageInputPart

			for _, content := range normalContents {
				switch content.ContentType {
				case "text":
					multiContent = append(multiContent, schema.MessageInputPart{
						Type: schema.MessagePartTypeText,
						Text: content.TextContent,
					})

				case "image_url":
					// 处理图片：检查文件是否存在，读取并转换为base64
					imagePart, err := h.processImageContent(content.MediaURL)
					if err != nil {
						g.Log().Errorf(gctx.New(), "Failed to process image %s: %v", content.MediaURL, err)
						// 图片处理失败，跳过该图片
						continue
					}
					multiContent = append(multiContent, imagePart)

				case "audio_url":
					// 处理音频：检查文件是否存在，读取并转换为base64
					audioPart, err := h.processAudioContent(content.MediaURL)
					if err != nil {
						g.Log().Errorf(gctx.New(), "Failed to process audio %s: %v", content.MediaURL, err)
						// 音频处理失败，跳过该音频
						continue
					}
					multiContent = append(multiContent, audioPart)

				case "video_url":
					// 处理视频：检查文件是否存在，读取并转换为base64
					videoPart, err := h.processVideoContent(content.MediaURL)
					if err != nil {
						g.Log().Errorf(gctx.New(), "Failed to process video %s: %v", content.MediaURL, err)
						// 视频处理失败，跳过该视频
						continue
					}
					multiContent = append(multiContent, videoPart)
				}
			}

			schemaMsg.UserInputMultiContent = multiContent
		} else if len(normalContents) == 1 {
			// 单个文本内容，使用Content字段
			schemaMsg.Content = normalContents[0].TextContent
		}

		// 处理 tool 内容：将其添加到 Extra.tool 数组中
		if len(toolContents) > 0 {
			toolResults := make([]map[string]interface{}, 0, len(toolContents))

			for _, toolContent := range toolContents {
				toolResult := map[string]interface{}{
					"content": toolContent.TextContent,
				}

				// 从 metadata 中提取工具信息
				if len(toolContent.Metadata) > 0 {
					var metadata map[string]interface{}
					if err := json.Unmarshal(toolContent.Metadata, &metadata); err == nil {
						if toolCallID, ok := metadata["tool_call_id"].(string); ok {
							toolResult["tool_call_id"] = toolCallID
						}
						if toolName, ok := metadata["tool_name"].(string); ok {
							toolResult["tool_name"] = toolName
						}
						if toolArgs, ok := metadata["tool_args"]; ok {
							toolResult["tool_args"] = toolArgs
						}
					}
				}

				toolResults = append(toolResults, toolResult)
			}

			schemaMsg.Extra["tool"] = toolResults
		}

		// 添加消息到结果
		result = append(result, schemaMsg)
	}

	return result, nil
}

// processImageContent 处理图片内容，将文件路径转换为base64 data URI
func (h *Manager) processImageContent(mediaURL string) (schema.MessageInputPart, error) {
	// 检查是否是文件路径
	if len(mediaURL) == 0 {
		return schema.MessageInputPart{}, errors.New(errors.ErrInvalidParameter, "empty media URL")
	}

	// 如果已经是data URI或HTTP URL，直接返回
	if strings.HasPrefix(mediaURL, "data:") || strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://") {
		return schema.MessageInputPart{
			Type: schema.MessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL: &mediaURL,
				},
				Detail: schema.ImageDetailAuto,
			},
		}, nil
	}
	cwd, _ := os.Getwd()
	// 检查文件路径是否为绝对路径，如果是相对路径则使用当前工作目录
	filePath := mediaURL
	if !filepath.IsAbs(mediaURL) {
		// 相对路径，使用当前工作目录拼接
		filePath = filepath.Join(cwd, mediaURL)
	}

	// 检查文件是否存在
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// 返回一个占位符表示图片不可用,而不是返回错误,避免影响整个对话加载
		return schema.MessageInputPart{
			Type: schema.MessagePartTypeText,
			Text: fmt.Sprintf("[图片不可用: %s]", filepath.Base(mediaURL)),
		}, nil
	}
	// 读取文件（使用处理后的绝对路径）
	data, err := os.ReadFile(filePath)
	if err != nil {
		g.Log().Errorf(gctx.New(), "[processImageContent] Failed to read file: %v", err)
		return schema.MessageInputPart{}, errors.Newf(errors.ErrFileReadFailed, "failed to read image file: %v", err)
	}

	// 获取MIME类型
	ext := filepath.Ext(mediaURL)
	mimeType := getMimeTypeFromExt(ext)

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// 构造data URI
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	return schema.MessageInputPart{
		Type: schema.MessagePartTypeImageURL,
		Image: &schema.MessageInputImage{
			MessagePartCommon: schema.MessagePartCommon{
				URL: &dataURI,
			},
			Detail: schema.ImageDetailAuto,
		},
	}, nil
}

// processAudioContent 处理音频内容，将文件路径转换为base64 data URI
func (h *Manager) processAudioContent(mediaURL string) (schema.MessageInputPart, error) {
	// 检查是否是文件路径
	if len(mediaURL) == 0 {
		return schema.MessageInputPart{}, errors.New(errors.ErrInvalidParameter, "empty media URL")
	}

	// 如果已经是data URI或HTTP URL，直接返回
	if strings.HasPrefix(mediaURL, "data:") || strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://") {
		return schema.MessageInputPart{
			Type: schema.MessagePartTypeAudioURL,
			Audio: &schema.MessageInputAudio{
				MessagePartCommon: schema.MessagePartCommon{
					URL: &mediaURL,
				},
			},
		}, nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(mediaURL); os.IsNotExist(err) {
		return schema.MessageInputPart{}, errors.Newf(errors.ErrFileReadFailed, "audio file not found: %s", mediaURL)
	}

	// 读取文件
	data, err := os.ReadFile(mediaURL)
	if err != nil {
		return schema.MessageInputPart{}, errors.Newf(errors.ErrFileReadFailed, "failed to read audio file: %v", err)
	}

	// 获取MIME类型
	ext := filepath.Ext(mediaURL)
	mimeType := getMimeTypeFromExt(ext)

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// 构造data URI
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	return schema.MessageInputPart{
		Type: schema.MessagePartTypeAudioURL,
		Audio: &schema.MessageInputAudio{
			MessagePartCommon: schema.MessagePartCommon{
				URL: &dataURI,
			},
		},
	}, nil
}

// processVideoContent 处理视频内容，将文件路径转换为base64 data URI
func (h *Manager) processVideoContent(mediaURL string) (schema.MessageInputPart, error) {
	// 检查是否是文件路径
	if len(mediaURL) == 0 {
		return schema.MessageInputPart{}, errors.New(errors.ErrInvalidParameter, "empty media URL")
	}

	// 如果已经是data URI或HTTP URL，直接返回
	if strings.HasPrefix(mediaURL, "data:") || strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://") {
		return schema.MessageInputPart{
			Type: schema.MessagePartTypeVideoURL,
			Video: &schema.MessageInputVideo{
				MessagePartCommon: schema.MessagePartCommon{
					URL: &mediaURL,
				},
			},
		}, nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(mediaURL); os.IsNotExist(err) {
		return schema.MessageInputPart{}, errors.Newf(errors.ErrFileReadFailed, "video file not found: %s", mediaURL)
	}

	// 读取文件
	data, err := os.ReadFile(mediaURL)
	if err != nil {
		return schema.MessageInputPart{}, errors.Newf(errors.ErrFileReadFailed, "failed to read video file: %v", err)
	}

	// 获取MIME类型
	ext := filepath.Ext(mediaURL)
	mimeType := getMimeTypeFromExt(ext)

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// 构造data URI
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	return schema.MessageInputPart{
		Type: schema.MessagePartTypeVideoURL,
		Video: &schema.MessageInputVideo{
			MessagePartCommon: schema.MessagePartCommon{
				URL: &dataURI,
			},
		},
	}, nil
}

// getMimeTypeFromExt 根据文件扩展名获取MIME类型
func getMimeTypeFromExt(ext string) string {
	mimeTypes := map[string]string{
		// 图片格式
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".tiff": "image/tiff",

		// 音频格式
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".flac": "audio/flac",
		".aac":  "audio/aac",
		".ogg":  "audio/ogg",
		".m4a":  "audio/mp4",
		".wma":  "audio/x-ms-wma",

		// 视频格式
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",
		".webm": "video/webm",
		".m4v":  "video/mp4",
		".mpeg": "video/mpeg",
		".mpg":  "video/mpeg",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// ensureConversationExists 确保对话存在
func (h *Manager) ensureConversationExists(convID string) error {
	conversation, err := dao.Conversation.GetByConvID(nil, convID)
	if err != nil {
		return err
	}

	if conversation == nil {
		now := time.Now()
		conversation := &gormModel.Conversation{
			ConvID:           convID,
			UserID:           "default_user", // 默认用户ID，实际使用时应从上下文获取
			Title:            "New Conversation",
			ModelID:          "default_model", // 默认模型名
			ConversationType: "text",
			Status:           "active",
			CreateTime:       &now,
			UpdateTime:       &now,
		}
		return dao.Conversation.Create(nil, conversation)
	}

	return nil
}

// generateMessageID 生成消息ID
func generateMessageID() string {
	return uuid.New().String()
}

// ========== 异步消息保存器 ==========

// SaveTask 消息保存任务
type SaveTask struct {
	Message *MessageWithMetrics
	ConvID  string
	Result  chan error
}

// SaveMetadataTask 带元数据的消息保存任务
type SaveMetadataTask struct {
	Message    *schema.Message
	ConvID     string
	Metadata   map[string]interface{}
	CreateTime *time.Time
	Result     chan error
}

// AsyncMessageSaver 异步消息保存器
type AsyncMessageSaver struct {
	db                *gorm.DB
	taskQueue         chan *SaveTask
	metadataTaskQueue chan *SaveMetadataTask
	workerPool        int
	wg                sync.WaitGroup
	ctx               context.Context
	cancel            context.CancelFunc
}

// NewAsyncMessageSaver 创建异步消息保存器
func NewAsyncMessageSaver(workerPool int) *AsyncMessageSaver {
	if workerPool <= 0 {
		workerPool = 5 // 默认5个worker
	}

	ctx, cancel := context.WithCancel(gctx.New())
	saver := &AsyncMessageSaver{
		db:                dao.GetDB(),
		taskQueue:         make(chan *SaveTask, 200),         // 缓冲队列
		metadataTaskQueue: make(chan *SaveMetadataTask, 200), // 元数据任务队列
		workerPool:        workerPool,
		ctx:               ctx,
		cancel:            cancel,
	}

	// 启动worker pool
	saver.start()

	return saver
}

// start 启动worker pool
func (s *AsyncMessageSaver) start() {
	// 启动处理 SaveTask 的 worker
	for i := 0; i < s.workerPool; i++ {
		s.wg.Add(1)
		go s.worker()
	}

	// 启动处理 SaveMetadataTask 的 worker
	for i := 0; i < s.workerPool; i++ {
		s.wg.Add(1)
		go s.metadataWorker()
	}
}

// worker 处理消息保存任务
func (s *AsyncMessageSaver) worker() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case task, ok := <-s.taskQueue:
			if !ok {
				return
			}
			// 处理消息保存
			err := s.saveMessageSync(task.Message, task.ConvID)
			if task.Result != nil {
				task.Result <- err
				close(task.Result)
			}
		}
	}
}

// metadataWorker 处理带元数据的消息保存任务
func (s *AsyncMessageSaver) metadataWorker() {
	defer s.wg.Done()

	historyManager := &Manager{db: s.db}

	for {
		select {
		case <-s.ctx.Done():
			return
		case task, ok := <-s.metadataTaskQueue:
			if !ok {
				return
			}
			// 处理带元数据的消息保存
			err := historyManager.SaveMessageWithMetadataSync(task.Message, task.ConvID, task.Metadata, task.CreateTime)
			if task.Result != nil {
				task.Result <- err
				close(task.Result)
			}
		}
	}
}

// saveMessageSync 同步保存消息（worker使用）
func (s *AsyncMessageSaver) saveMessageSync(message *MessageWithMetrics, convID string) error {
	// 确保对话存在
	if err := s.ensureConversationExists(convID); err != nil {
		return err
	}

	now := time.Now()

	// 处理工具调用
	var toolCallsJSON gormModel.JSON
	if message.ToolCalls != nil {
		data, err := json.Marshal(message.ToolCalls)
		if err != nil {
			g.Log().Errorf(gctx.New(), "failed to marshal tool calls: %v", err)
		} else {
			toolCallsJSON = data
		}
	}

	// 创建消息记录
	msg := &gormModel.Message{
		MsgID:      generateMessageID(),
		ConvID:     convID,
		Role:       string(message.Role),
		CreateTime: &now,
		TokensUsed: message.TokensUsed,
		LatencyMs:  message.LatencyMs,
		TraceID:    message.TraceID,
		ToolCalls:  toolCallsJSON,
	}

	// 处理内容块
	var contents []*gormModel.MessageContent
	content := &gormModel.MessageContent{
		ContentType: "text",
		TextContent: message.Content,
		SortOrder:   0,
		CreateTime:  &now,
	}
	contents = append(contents, content)

	return dao.Message.CreateWithContents(nil, msg, contents)
}

// SaveMessageAsync 异步保存消息
func (s *AsyncMessageSaver) SaveMessageAsync(message *MessageWithMetrics, convID string) {
	task := &SaveTask{
		Message: message,
		ConvID:  convID,
		Result:  nil, // 不需要结果通知
	}

	select {
	case s.taskQueue <- task:
		// 任务提交成功
	default:
		// 队列满了，记录警告但不阻塞
		g.Log().Warning(gctx.New(), "Message save queue is full, message may be lost")
	}
}

// SaveMessageWithMetadataAsync 异步保存带元数据的消息
func (s *AsyncMessageSaver) SaveMessageWithMetadataAsync(task *SaveMetadataTask) {
	select {
	case s.metadataTaskQueue <- task:
		// 任务提交成功
	default:
		// 队列满了，记录警告但不阻塞
		g.Log().Warning(gctx.New(), "Metadata message save queue is full, message may be lost")
	}
}

// ensureConversationExists 确保对话存在
func (s *AsyncMessageSaver) ensureConversationExists(convID string) error {
	conversation, err := dao.Conversation.GetByConvID(nil, convID)
	if err != nil {
		return err
	}

	if conversation == nil {
		now := time.Now()
		conversation := &gormModel.Conversation{
			ConvID:           convID,
			UserID:           "default_user",
			Title:            "New Conversation",
			ModelID:          "default_model",
			ConversationType: "text",
			Status:           "active",
			CreateTime:       &now,
			UpdateTime:       &now,
		}
		return dao.Conversation.Create(nil, conversation)
	}

	return nil
}

// Shutdown 关闭异步保存器
func (s *AsyncMessageSaver) Shutdown() {
	s.cancel()
	close(s.taskQueue)
	s.wg.Wait()
}

// GetQueueSize 获取当前队列大小
func (s *AsyncMessageSaver) GetQueueSize() int {
	return len(s.taskQueue)
}

// 全局异步保存器实例
var globalAsyncSaver *AsyncMessageSaver
var saverOnce sync.Once

// GetGlobalAsyncSaver 获取全局异步保存器
func GetGlobalAsyncSaver() *AsyncMessageSaver {
	saverOnce.Do(func() {
		globalAsyncSaver = NewAsyncMessageSaver(5)
	})
	return globalAsyncSaver
}

// DeleteConversationHistory 删除指定会话的所有消息历史
func (h *Manager) DeleteConversationHistory(ctx context.Context, convID string) error {
	// 获取该会话的所有消息
	messages, _, err := dao.Message.ListByConvID(ctx, convID, 1, 10000) // 假设最多10000条消息
	if err != nil {
		g.Log().Errorf(ctx, "查询会话消息失败: %v", err)
		return errors.Newf(errors.ErrDatabaseQuery, "failed to query messages: %v", err)
	}

	// 如果没有消息，直接返回
	if len(messages) == 0 {
		return nil
	}

	// 收集所有消息ID
	msgIDs := make([]string, 0, len(messages))
	for _, msg := range messages {
		msgIDs = append(msgIDs, msg.MsgID)
	}

	// 批量删除消息内容
	if err := dao.MessageContent.BatchDeleteByMsgIDs(ctx, msgIDs); err != nil {
		g.Log().Warningf(ctx, "批量删除消息内容失败: %v", err)
		// 继续执行，不阻断流程
	}

	// 批量删除消息
	if err := dao.Message.BatchDeleteByConvID(ctx, convID); err != nil {
		g.Log().Errorf(ctx, "批量删除消息失败: %v", err)
		return errors.Newf(errors.ErrDatabaseDelete, "failed to delete messages: %v", err)
	}

	g.Log().Infof(ctx, "成功删除会话 %s 的 %d 条消息", convID, len(messages))
	return nil
}
