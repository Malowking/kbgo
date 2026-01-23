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
	"github.com/Malowking/kbgo/core/schema"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
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

// SaveMessage 异步保存消息，支持自定义时间戳和trace_id
func (h *Manager) SaveMessage(message *schema.Message, convID string, metadata map[string]interface{}, createTime *time.Time, traceID ...string) error {
	// 使用全局异步保存器
	asyncSaver := GetGlobalAsyncSaver()

	// 如果没有提供时间戳，使用当前时间
	if createTime == nil {
		now := time.Now()
		createTime = &now
	}

	// 提取 traceID（可选参数）
	var tid string
	if len(traceID) > 0 {
		tid = traceID[0]
	}

	// 构建保存任务
	task := &SaveMetadataTask{
		Message:    message,
		ConvID:     convID,
		Metadata:   metadata,
		CreateTime: createTime,
		TraceID:    tid,
		Result:     nil, // 不等待结果
	}

	// 异步保存
	asyncSaver.SaveMessageWithMetadataAsync(task)

	return nil
}

// SaveMessageWithMetadataSync 同步保存带元数据的消息
func (h *Manager) SaveMessageWithMetadataSync(message *schema.Message, convID string, metadata map[string]interface{}, createTime *time.Time, traceID string) error {
	// 确保对话存在
	if err := h.ensureConversationExists(convID); err != nil {
		return err
	}

	// 如果没有提供时间戳，使用当前时间
	if createTime == nil {
		now := time.Now()
		createTime = &now
	}

	// 提取文本内容和文件信息
	var textContent string
	var files []map[string]interface{}

	// 优先处理 UserInputMultiContent（新版多模态字段）
	if len(message.UserInputMultiContent) > 0 {
		for _, part := range message.UserInputMultiContent {
			switch part.Type {
			case schema.MessagePartTypeText:
				textContent = part.Text

			case schema.MessagePartTypeImageURL:
				if part.Image != nil && part.Image.URL != nil {
					files = append(files, map[string]interface{}{
						"type": "image",
						"path": *part.Image.URL,
					})
				}

			case schema.MessagePartTypeAudioURL:
				if part.Audio != nil && part.Audio.URL != nil {
					files = append(files, map[string]interface{}{
						"type": "audio",
						"path": *part.Audio.URL,
					})
				}

			case schema.MessagePartTypeVideoURL:
				if part.Video != nil && part.Video.URL != nil {
					files = append(files, map[string]interface{}{
						"type": "video",
						"path": *part.Video.URL,
					})
				}
			}
		}
	} else if message.Content != "" {
		// 普通文本消息
		textContent = message.Content
	}

	// 构建元数据
	finalMetadata := make(map[string]interface{})
	if metadata != nil {
		for k, v := range metadata {
			finalMetadata[k] = v
		}
	}
	if len(files) > 0 {
		finalMetadata["files"] = files
	}

	// 处理元数据
	var metadataJSON gormModel.JSON
	if len(finalMetadata) > 0 {
		data, err := json.Marshal(finalMetadata)
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
		Content:    textContent,
		ToolCallID: message.ToolCallID,
		CreateTime: createTime,
		Metadata:   metadataJSON,
		ToolCalls:  toolCallsJSON,
		TraceID:    traceID,
	}

	// 直接保存消息
	return h.db.Create(msg).Error
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

	// 转换为 schema.Message
	result := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		schemaMsg := &schema.Message{
			Role:       schema.RoleType(msg.Role),
			ToolCallID: msg.ToolCallID,
			Extra:      make(map[string]any),
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

		// 从 Content 字段读取文本内容
		schemaMsg.Content = msg.Content

		// 从 Metadata 字段读取文件信息
		if len(msg.Metadata) > 0 {
			var metadata map[string]interface{}
			if err := json.Unmarshal(msg.Metadata, &metadata); err == nil {
				if filesData, ok := metadata["files"]; ok {
					// 将文件信息转换为 UserInputMultiContent
					if files, ok := filesData.([]interface{}); ok {
						var multiContent []schema.MessageInputPart

						// 如果有文本内容，先添加文本
						if msg.Content != "" {
							multiContent = append(multiContent, schema.MessageInputPart{
								Type: schema.MessagePartTypeText,
								Text: msg.Content,
							})
						}

						// 添加文件
						for _, fileData := range files {
							if file, ok := fileData.(map[string]interface{}); ok {
								fileType, _ := file["type"].(string)
								filePath, _ := file["path"].(string)

								switch fileType {
								case "image":
									imagePart, err := h.processImageContent(filePath)
									if err != nil {
										g.Log().Errorf(gctx.New(), "Failed to process image %s: %v", filePath, err)
										continue
									}
									multiContent = append(multiContent, imagePart)

								case "audio":
									audioPart, err := h.processAudioContent(filePath)
									if err != nil {
										g.Log().Errorf(gctx.New(), "Failed to process audio %s: %v", filePath, err)
										continue
									}
									multiContent = append(multiContent, audioPart)

								case "video":
									videoPart, err := h.processVideoContent(filePath)
									if err != nil {
										g.Log().Errorf(gctx.New(), "Failed to process video %s: %v", filePath, err)
										continue
									}
									multiContent = append(multiContent, videoPart)
								}
							}
						}

						if len(multiContent) > 0 {
							schemaMsg.UserInputMultiContent = multiContent
							schemaMsg.Content = "" // 清空 Content，使用 MultiContent
						}
					}
				}
			}
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
	TraceID    string
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
			err := historyManager.SaveMessageWithMetadataSync(task.Message, task.ConvID, task.Metadata, task.CreateTime, task.TraceID)
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
		Content:    message.Content,
		ToolCallID: message.ToolCallID,
		CreateTime: &now,
		TokensUsed: message.TokensUsed,
		LatencyMs:  message.LatencyMs,
		TraceID:    message.TraceID,
		ToolCalls:  toolCallsJSON,
	}

	// 直接保存消息
	return s.db.Create(msg).Error
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

	// 批量删除消息
	if err := dao.Message.BatchDeleteByConvID(ctx, convID); err != nil {
		g.Log().Errorf(ctx, "批量删除消息失败: %v", err)
		return errors.Newf(errors.ErrDatabaseDelete, "failed to delete messages: %v", err)
	}

	g.Log().Infof(ctx, "成功删除会话 %s 的 %d 条消息", convID, len(messages))
	return nil
}
