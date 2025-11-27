package common

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// MultimodalMessageBuilder 多模态消息构建器
type MultimodalMessageBuilder struct{}

// NewMultimodalMessageBuilder 创建多模态消息构建器
func NewMultimodalMessageBuilder() *MultimodalMessageBuilder {
	return &MultimodalMessageBuilder{}
}

// ContentPart 表示消息内容的一部分
type ContentPart struct {
	Type     string                 `json:"type"` // "text", "image_url", "audio_url", "video_url"
	Text     string                 `json:"text,omitempty"`
	ImageURL map[string]interface{} `json:"image_url,omitempty"`
	AudioURL map[string]interface{} `json:"audio_url,omitempty"`
	VideoURL map[string]interface{} `json:"video_url,omitempty"`
}

// BuildMultimodalMessage 构建多模态消息
// 根据不同的文件类型，构建符合OpenAI多模态API格式的消息
func (b *MultimodalMessageBuilder) BuildMultimodalMessage(
	text string,
	files []*MultimodalFile,
	useBase64 bool,
) (*schema.Message, error) {

	// 如果没有文件，返回普通文本消息
	if len(files) == 0 {
		return &schema.Message{
			Role:    schema.User,
			Content: text,
		}, nil
	}

	// 构建多模态内容
	var contentParts []ContentPart

	// 添加文本部分
	if text != "" {
		contentParts = append(contentParts, ContentPart{
			Type: "text",
			Text: text,
		})
	}

	// 添加文件部分
	for _, file := range files {
		var part ContentPart
		var err error

		switch file.FileType {
		case FileTypeImage:
			part, err = b.buildImagePart(file, useBase64)
		case FileTypeAudio:
			part, err = b.buildAudioPart(file, useBase64)
		case FileTypeVideo:
			part, err = b.buildVideoPart(file, useBase64)
		default:
			// 其他类型文件作为文本描述
			part = ContentPart{
				Type: "text",
				Text: fmt.Sprintf("[File: %s]", file.FileName),
			}
		}

		if err != nil {
			g.Log().Errorf(nil, "Failed to build content part for file %s: %v", file.FileName, err)
			continue
		}

		contentParts = append(contentParts, part)
	}

	// 构建消息
	// 注意：schema.Message 的 Content 字段是 string 类型
	// 对于多模态内容，我们需要将其序列化为JSON字符串
	// 或者使用 MultiContent 字段（如果支持）
	message := &schema.Message{
		Role: schema.User,
		// 将 contentParts 存储到 Extra 字段中
		Extra: map[string]any{
			"multimodal_contents": contentParts,
		},
	}

	// 如果只有文本，直接使用 Content 字段
	if len(contentParts) == 1 && contentParts[0].Type == "text" {
		message.Content = contentParts[0].Text
	} else {
		// 多模态内容，将文本部分提取到 Content
		message.Content = text
	}

	return message, nil
}

// buildImagePart 构建图片内容部分
func (b *MultimodalMessageBuilder) buildImagePart(file *MultimodalFile, useBase64 bool) (ContentPart, error) {
	if useBase64 {
		// 读取文件并转换为base64
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			return ContentPart{}, fmt.Errorf("failed to read image file: %w", err)
		}

		// 获取MIME类型
		ext := filepath.Ext(file.FileName)
		mimeType := getMimeType(ext)

		base64Data := base64.StdEncoding.EncodeToString(data)
		dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

		return ContentPart{
			Type: "image_url",
			ImageURL: map[string]interface{}{
				"url": dataURL,
			},
		}, nil
	}

	// 使用文件URL
	return ContentPart{
		Type: "image_url",
		ImageURL: map[string]interface{}{
			"url": file.RelativePath,
		},
	}, nil
}

// buildAudioPart 构建音频内容部分
func (b *MultimodalMessageBuilder) buildAudioPart(file *MultimodalFile, useBase64 bool) (ContentPart, error) {
	if useBase64 {
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			return ContentPart{}, fmt.Errorf("failed to read audio file: %w", err)
		}

		ext := filepath.Ext(file.FileName)
		mimeType := getMimeType(ext)

		base64Data := base64.StdEncoding.EncodeToString(data)
		dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

		return ContentPart{
			Type: "audio_url",
			AudioURL: map[string]interface{}{
				"url": dataURL,
			},
		}, nil
	}

	return ContentPart{
		Type: "audio_url",
		AudioURL: map[string]interface{}{
			"url": file.RelativePath,
		},
	}, nil
}

// buildVideoPart 构建视频内容部分
func (b *MultimodalMessageBuilder) buildVideoPart(file *MultimodalFile, useBase64 bool) (ContentPart, error) {
	// 视频文件通常较大，不建议使用base64
	// 这里只支持URL方式
	return ContentPart{
		Type: "video_url",
		VideoURL: map[string]interface{}{
			"url": file.RelativePath,
		},
	}, nil
}

// getMimeType 根据文件扩展名获取MIME类型
func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		// 图片
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".tiff": "image/tiff",

		// 音频
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".flac": "audio/flac",
		".aac":  "audio/aac",
		".ogg":  "audio/ogg",
		".m4a":  "audio/mp4",
		".wma":  "audio/x-ms-wma",

		// 视频
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

// ExtractMultimodalContent 从消息中提取多模态内容
func (b *MultimodalMessageBuilder) ExtractMultimodalContent(message *schema.Message) ([]ContentPart, bool) {
	if message.Extra == nil {
		return nil, false
	}

	if contents, ok := message.Extra["multimodal_contents"]; ok {
		if parts, ok := contents.([]ContentPart); ok {
			return parts, true
		}
	}

	return nil, false
}
