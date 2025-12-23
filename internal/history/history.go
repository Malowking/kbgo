package history

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Malowking/kbgo/core/errors"
	internalCache "github.com/Malowking/kbgo/internal/cache"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageWithContents 带内容块的消息结构
type MessageWithContents struct {
	*gormModel.Message // 包含 msg_id, role, tool_calls 等
	Contents           []gormModel.MessageContent
}

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

// SaveMessage 保存消息
func (h *Manager) SaveMessage(message *schema.Message, convID string) error {
	return h.SaveMessageWithMetadata(message, convID, nil)
}

// SaveMessageWithMetrics 保存带指标的消息（异步）
func (h *Manager) SaveMessageWithMetrics(message *MessageWithMetrics, convID string) error {
	// 使用全局异步保存器
	asyncSaver := GetGlobalAsyncSaver()

	// 异步保存，不等待结果（提升性能）
	asyncSaver.SaveMessageAsync(message, convID)

	return nil
}

// SaveMessageWithMetricsSync 保存带指标的消息（同步）
func (h *Manager) SaveMessageWithMetricsSync(message *MessageWithMetrics, convID string) error {
	// 确保对话存在
	if err := h.ensureConversationExists(convID); err != nil {
		return err
	}

	now := time.Now()

	// 处理工具调用
	var toolCallsJSON gormModel.JSON
	if message.ToolCalls != nil {
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

// SaveMessageWithMetadata 保存带元数据的消息
func (h *Manager) SaveMessageWithMetadata(message *schema.Message, convID string, metadata map[string]interface{}) error {
	// 确保对话存在
	if err := h.ensureConversationExists(convID); err != nil {
		return err
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

	now := time.Now()
	// 创建消息记录
	msg := &gormModel.Message{
		MsgID:      generateMessageID(),
		ConvID:     convID,
		Role:       string(message.Role),
		CreateTime: &now,
		Metadata:   metadataJSON,
	}

	// 处理内容块 - 支持多模态内容
	var contents []*gormModel.MessageContent

	// 优先处理 UserInputMultiContent（新版多模态字段）
	if len(message.UserInputMultiContent) > 0 {
		for i, part := range message.UserInputMultiContent {
			content := &gormModel.MessageContent{
				SortOrder:  i,
				CreateTime: &now,
			}

			switch part.Type {
			case schema.ChatMessagePartTypeText:
				content.ContentType = "text"
				content.TextContent = part.Text

			case schema.ChatMessagePartTypeImageURL:
				if part.Image != nil {
					content.ContentType = "image_url"
					// 存储文件路径到media_url
					if part.Image.URL != nil {
						content.MediaURL = *part.Image.URL
					}
				}

			case schema.ChatMessagePartTypeAudioURL:
				content.ContentType = "audio_url"
				if part.Audio != nil && part.Audio.URL != nil {
					content.MediaURL = *part.Audio.URL
				}

			case schema.ChatMessagePartTypeVideoURL:
				content.ContentType = "video_url"
				if part.Video != nil && part.Video.URL != nil {
					content.MediaURL = *part.Video.URL
				}
			}

			contents = append(contents, content)
		}
	} else if len(message.MultiContent) > 0 {
		// 处理 MultiContent（旧版多模态字段）
		for i, part := range message.MultiContent {
			content := &gormModel.MessageContent{
				SortOrder:  i,
				CreateTime: &now,
			}

			switch part.Type {
			case schema.ChatMessagePartTypeText:
				content.ContentType = "text"
				content.TextContent = part.Text

			case schema.ChatMessagePartTypeImageURL:
				content.ContentType = "image_url"
				if part.ImageURL != nil {
					content.MediaURL = part.ImageURL.URL
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
			CreateTime:  &now,
		}
		contents = append(contents, content)
	}

	// 如果没有任何内容，至少保存一个空文本内容
	if len(contents) == 0 {
		content := &gormModel.MessageContent{
			ContentType: "text",
			TextContent: "",
			SortOrder:   0,
			CreateTime:  &now,
		}
		contents = append(contents, content)
	}

	// 使用缓存层保存消息
	messageCache := internalCache.GetMessageCache()
	if messageCache != nil {
		// 使用缓存层（异步刷盘到数据库）
		return messageCache.SaveMessage(context.Background(), msg, contents)
	}

	// 缓存层不可用，直接写数据库
	return dao.Message.CreateWithContents(nil, msg, contents)
}

// GetHistory 获取聊天历史
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
	result := make([]*schema.Message, len(messages))
	for i, msg := range messages {
		// 获取该消息的内容块
		msgContents := contentMap[msg.MsgID]

		schemaMsg := &schema.Message{
			Role: schema.RoleType(msg.Role),
		}

		// 如果有多个内容块或包含非文本内容，构建MultiContent
		if len(msgContents) > 1 || (len(msgContents) == 1 && msgContents[0].ContentType != "text") {
			var multiContent []schema.ChatMessagePart

			for _, content := range msgContents {
				switch content.ContentType {
				case "text":
					multiContent = append(multiContent, schema.ChatMessagePart{
						Type: schema.ChatMessagePartTypeText,
						Text: content.TextContent,
					})

				case "image_url":
					// 处理图片：检查文件是否存在，读取并转换为base64
					imagePart, err := h.processImageContent(content.MediaURL)
					if err != nil {
						g.Log().Errorf(context.Background(), "Failed to process image %s: %v", content.MediaURL, err)
						// 图片处理失败，跳过该图片
						continue
					}
					multiContent = append(multiContent, imagePart)

				case "audio_url":
					// 处理音频：检查文件是否存在，读取并转换为base64
					audioPart, err := h.processAudioContent(content.MediaURL)
					if err != nil {
						g.Log().Errorf(context.Background(), "Failed to process audio %s: %v", content.MediaURL, err)
						// 音频处理失败，跳过该音频
						continue
					}
					multiContent = append(multiContent, audioPart)

				case "video_url":
					// 处理视频：检查文件是否存在，读取并转换为base64
					videoPart, err := h.processVideoContent(content.MediaURL)
					if err != nil {
						g.Log().Errorf(context.Background(), "Failed to process video %s: %v", content.MediaURL, err)
						// 视频处理失败，跳过该视频
						continue
					}
					multiContent = append(multiContent, videoPart)
				}
			}

			schemaMsg.MultiContent = multiContent
		} else if len(msgContents) == 1 {
			// 单个文本内容，使用Content字段
			schemaMsg.Content = msgContents[0].TextContent
		}

		result[i] = schemaMsg
	}

	return result, nil
}

// processImageContent 处理图片内容，将文件路径转换为base64 data URI
func (h *Manager) processImageContent(mediaURL string) (schema.ChatMessagePart, error) {
	// 检查是否是文件路径
	if len(mediaURL) == 0 {
		return schema.ChatMessagePart{}, errors.New(errors.ErrInvalidParameter, "empty media URL")
	}

	// 如果已经是data URI或HTTP URL，直接返回
	if strings.HasPrefix(mediaURL, "data:") || strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://") {
		return schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeImageURL,
			ImageURL: &schema.ChatMessageImageURL{
				URL:    mediaURL,
				Detail: schema.ImageURLDetailAuto,
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
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// 返回一个占位符表示图片不可用,而不是返回错误,避免影响整个对话加载
		return schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeText,
			Text: fmt.Sprintf("[图片不可用: %s]", filepath.Base(mediaURL)),
		}, nil
	}
	g.Log().Debugf(context.Background(), "[processImageContent] File found, size=%d bytes", fileInfo.Size())

	// 读取文件（使用处理后的绝对路径）
	data, err := os.ReadFile(filePath)
	if err != nil {
		g.Log().Errorf(context.Background(), "[processImageContent] Failed to read file: %v", err)
		return schema.ChatMessagePart{}, errors.Newf(errors.ErrFileReadFailed, "failed to read image file: %v", err)
	}

	// 获取MIME类型
	ext := filepath.Ext(mediaURL)
	mimeType := getMimeTypeFromExt(ext)

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// 构造data URI
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	return schema.ChatMessagePart{
		Type: schema.ChatMessagePartTypeImageURL,
		ImageURL: &schema.ChatMessageImageURL{
			URL:    dataURI,
			Detail: schema.ImageURLDetailAuto,
		},
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// processAudioContent 处理音频内容，将文件路径转换为base64 data URI
func (h *Manager) processAudioContent(mediaURL string) (schema.ChatMessagePart, error) {
	// 检查是否是文件路径
	if len(mediaURL) == 0 {
		return schema.ChatMessagePart{}, errors.New(errors.ErrInvalidParameter, "empty media URL")
	}

	// 如果已经是data URI或HTTP URL，直接返回
	if strings.HasPrefix(mediaURL, "data:") || strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://") {
		return schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeAudioURL,
			AudioURL: &schema.ChatMessageAudioURL{
				URL: mediaURL,
			},
		}, nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(mediaURL); os.IsNotExist(err) {
		return schema.ChatMessagePart{}, errors.Newf(errors.ErrFileReadFailed, "audio file not found: %s", mediaURL)
	}

	// 读取文件
	data, err := os.ReadFile(mediaURL)
	if err != nil {
		return schema.ChatMessagePart{}, errors.Newf(errors.ErrFileReadFailed, "failed to read audio file: %v", err)
	}

	// 获取MIME类型
	ext := filepath.Ext(mediaURL)
	mimeType := getMimeTypeFromExt(ext)

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// 构造data URI
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	return schema.ChatMessagePart{
		Type: schema.ChatMessagePartTypeAudioURL,
		AudioURL: &schema.ChatMessageAudioURL{
			URL: dataURI,
		},
	}, nil
}

// processVideoContent 处理视频内容，将文件路径转换为base64 data URI
func (h *Manager) processVideoContent(mediaURL string) (schema.ChatMessagePart, error) {
	// 检查是否是文件路径
	if len(mediaURL) == 0 {
		return schema.ChatMessagePart{}, errors.New(errors.ErrInvalidParameter, "empty media URL")
	}

	// 如果已经是data URI或HTTP URL，直接返回
	if strings.HasPrefix(mediaURL, "data:") || strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://") {
		return schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeVideoURL,
			VideoURL: &schema.ChatMessageVideoURL{
				URL: mediaURL,
			},
		}, nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(mediaURL); os.IsNotExist(err) {
		return schema.ChatMessagePart{}, errors.Newf(errors.ErrFileReadFailed, "video file not found: %s", mediaURL)
	}

	// 读取文件
	data, err := os.ReadFile(mediaURL)
	if err != nil {
		return schema.ChatMessagePart{}, errors.Newf(errors.ErrFileReadFailed, "failed to read video file: %v", err)
	}

	// 获取MIME类型
	ext := filepath.Ext(mediaURL)
	mimeType := getMimeTypeFromExt(ext)

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// 构造data URI
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	return schema.ChatMessagePart{
		Type: schema.ChatMessagePartTypeVideoURL,
		VideoURL: &schema.ChatMessageVideoURL{
			URL: dataURI,
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

// GetConversationHistory 获取会话历史消息
func (h *Manager) GetConversationHistory(convID string) ([]MessageWithContents, error) {
	var msgs []MessageWithContents
	err := dao.GetDB().
		Where("conv_id = ?", convID).
		Order("create_time ASC").
		Find(&msgs).
		Error

	if err != nil {
		return nil, err
	}

	// 预加载消息内容
	for i := range msgs {
		var contents []gormModel.MessageContent
		err := dao.GetDB().
			Where("msg_id = ?", msgs[i].MsgID).
			Order("sort_order ASC").
			Find(&contents).
			Error

		if err != nil {
			return nil, err
		}

		msgs[i].Contents = contents
	}

	return msgs, nil
}

// BuildLLMMessages 将历史消息转换为LLM格式
func (h *Manager) BuildLLMMessages(history []MessageWithContents) []map[string]interface{} {
	var llmMsgs []map[string]interface{}

	for _, m := range history {
		// 处理 tool 消息
		if m.Role == "tool" {
			// tool 消息：content 是 JSON 字符串，需解析为文本描述或保留原样
			content := fmt.Sprintf("[Tool Result: %s]", m.Contents[0].TextContent)
			llmMsgs = append(llmMsgs, map[string]interface{}{
				"role":         "tool",
				"content":      content,
				"tool_call_id": m.ToolCallID,
			})
			continue
		}

		// 合并 message_contents 为纯文本
		var parts []string
		for _, c := range m.Contents {
			switch c.ContentType {
			case "text", "json_data", "tool_result":
				parts = append(parts, c.TextContent)
			case "image_url", "audio_url", "file_url":
				// 当前仅 LLM，无法理解媒体，转为文本描述
				parts = append(parts, fmt.Sprintf("[Uploaded file: %s]", extractFileName(c.MediaURL)))
			case "file_binary_ref":
				parts = append(parts, fmt.Sprintf("[File uploaded: %s]", c.StorageKey))
			}
		}
		content := strings.Join(parts, "\n")

		// 处理 assistant 的 tool_calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			// OpenAI 格式：content 可为空，tool_calls 单独字段
			msg := map[string]interface{}{
				"role":       "assistant",
				"content":    content,
				"tool_calls": m.ToolCalls, // 假设 ToolCalls 是 json.RawMessage 或 map
			}
			llmMsgs = append(llmMsgs, msg)
		} else {
			llmMsgs = append(llmMsgs, map[string]interface{}{
				"role":    m.Role,
				"content": content,
			})
		}
	}

	return llmMsgs
}

// TruncateMessagesByToken 根据token数量截断消息
func (h *Manager) TruncateMessagesByToken(messages []map[string]interface{}, maxTokens int, model string) []map[string]interface{} {
	// 粗略估算：1 token 4 英文字符 or 1.5 中文字符
	// 更准的方式：使用 tiktoken 库（见下方建议）
	totalTokens := 0
	startIdx := 0

	for i, msg := range messages {
		content, _ := msg["content"].(string)
		tokens := h.estimateTokenCount(content) + 10 // + role 开销
		if totalTokens+tokens > maxTokens {
			startIdx = i
			break
		}
		totalTokens += tokens
	}

	// 保证至少包含最后一条用户消息
	if startIdx >= len(messages) {
		startIdx = len(messages) - 1
	}

	return messages[startIdx:]
}

// estimateTokenCount 估算token数量
func (h *Manager) estimateTokenCount(text string) int {
	// 简化版：中文按 1.5 字/词，英文按 4 字/词
	chinese := utf8.RuneCountInString(regexp.MustCompile(`[\p{Han}]`).ReplaceAllString(text, ""))
	english := len(regexp.MustCompile(`[a-zA-Z0-9]+`).ReplaceAllString(text, "")) / 4
	return chinese + english
}

// extractFileName 从URL中提取文件名
func extractFileName(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
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
			ModelName:        "default_model", // 默认模型名
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

// GetMessageMetadata 获取消息的元数据
func (h *Manager) GetMessageMetadata(msgID string) (map[string]interface{}, error) {
	message, err := dao.Message.GetByMsgID(nil, msgID)
	if err != nil {
		return nil, err
	}

	if message == nil || len(message.Metadata) == 0 {
		return nil, nil
	}

	var metadata map[string]interface{}
	err = json.Unmarshal(message.Metadata, &metadata)
	if err != nil {
		return nil, errors.Newf(errors.ErrInternalError, "failed to unmarshal metadata: %v", err)
	}

	return metadata, nil
}

// ========== 异步消息保存器 ==========

// SaveTask 消息保存任务
type SaveTask struct {
	Message *MessageWithMetrics
	ConvID  string
	Result  chan error
}

// AsyncMessageSaver 异步消息保存器
type AsyncMessageSaver struct {
	db         *gorm.DB
	taskQueue  chan *SaveTask
	workerPool int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewAsyncMessageSaver 创建异步消息保存器
func NewAsyncMessageSaver(workerPool int) *AsyncMessageSaver {
	if workerPool <= 0 {
		workerPool = 5 // 默认5个worker
	}

	ctx, cancel := context.WithCancel(context.Background())
	saver := &AsyncMessageSaver{
		db:         dao.GetDB(),
		taskQueue:  make(chan *SaveTask, 200), // 缓冲队列
		workerPool: workerPool,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 启动worker pool
	saver.start()

	return saver
}

// start 启动worker pool
func (s *AsyncMessageSaver) start() {
	for i := 0; i < s.workerPool; i++ {
		s.wg.Add(1)
		go s.worker()
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
			g.Log().Errorf(context.Background(), "failed to marshal tool calls: %v", err)
		} else {
			toolCallsJSON = gormModel.JSON(data)
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

// SaveMessageAsync 异步保存消息（不等待结果）
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
		g.Log().Warning(context.Background(), "Message save queue is full, message may be lost")
	}
}

// SaveMessageAsyncWait 异步保存消息（等待结果）
func (s *AsyncMessageSaver) SaveMessageAsyncWait(ctx context.Context, message *MessageWithMetrics, convID string) error {
	task := &SaveTask{
		Message: message,
		ConvID:  convID,
		Result:  make(chan error, 1),
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.taskQueue <- task:
		// 任务提交成功，等待结果
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-task.Result:
			return err
		}
	default:
		// 队列满了，同步保存
		g.Log().Warning(ctx, "Message save queue is full, saving synchronously")
		return s.saveMessageSync(message, convID)
	}
}

// ensureConversationExists 确保对话存在（AsyncMessageSaver使用）
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
			ModelName:        "default_model",
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
