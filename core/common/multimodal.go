package common

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// MultimodalMessageBuilder 多模态消息构建器
type MultimodalMessageBuilder struct{}

// NewMultimodalMessageBuilder 创建多模态消息构建器
func NewMultimodalMessageBuilder() *MultimodalMessageBuilder {
	return &MultimodalMessageBuilder{}
}

// BuildMultimodalMessage 构建多模态消息
// 根据不同的文件类型，构建符合eino框架的多模态消息格式
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

	// 构建多模态内容 - 同时使用两种字段以兼容不同的模型
	var multiContent []schema.MessageInputPart
	var chatMessageParts []schema.ChatMessagePart

	// 添加文本部分
	if text != "" {
		multiContent = append(multiContent, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: text,
		})
		chatMessageParts = append(chatMessageParts, schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeText,
			Text: text,
		})
	}

	// 添加文件部分
	for _, file := range files {
		part, err := b.buildInputPart(file, useBase64)
		if err != nil {
			g.Log().Errorf(nil, "Failed to build content part for file %s: %v", file.FileName, err)
			continue
		}
		multiContent = append(multiContent, part)

		// 同时构建ChatMessagePart（用于MultiContent字段）
		chatPart, err := b.buildChatMessagePart(file, useBase64)
		if err == nil {
			chatMessageParts = append(chatMessageParts, chatPart)
		}
	}

	// 构建消息 - 同时使用UserInputMultiContent和MultiContent以提高兼容性
	message := &schema.Message{
		Role:                  schema.User,
		UserInputMultiContent: multiContent,
		MultiContent:          chatMessageParts, // 废弃字段，但某些模型可能需要
	}

	return message, nil
}

// buildInputPart 构建符合eino框架标准的MessageInputPart
func (b *MultimodalMessageBuilder) buildInputPart(file *MultimodalFile, useBase64 bool) (schema.MessageInputPart, error) {
	switch file.FileType {
	case FileTypeImage:
		return b.buildImageInputPart(file, useBase64)
	case FileTypeAudio:
		return b.buildAudioInputPart(file, useBase64)
	case FileTypeVideo:
		return b.buildVideoInputPart(file, useBase64)
	default:
		// 其他类型文件作为文本描述
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: fmt.Sprintf("[File: %s]", file.FileName),
		}, nil
	}
}

// buildChatMessagePart 构建ChatMessagePart（用于MultiContent字段）
func (b *MultimodalMessageBuilder) buildChatMessagePart(file *MultimodalFile, useBase64 bool) (schema.ChatMessagePart, error) {
	ext := filepath.Ext(file.FileName)
	mimeType := getMimeType(ext)

	switch file.FileType {
	case FileTypeImage:
		if useBase64 {
			data, err := os.ReadFile(file.FilePath)
			if err != nil {
				return schema.ChatMessagePart{}, errors.Newf(errors.ErrFileReadFailed, "failed to read image file %s: %v", file.FilePath, err)
			}
			base64Data := base64.StdEncoding.EncodeToString(data)
			return schema.ChatMessagePart{
				Type: schema.ChatMessagePartTypeImageURL,
				ImageURL: &schema.ChatMessageImageURL{
					URL:    fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data),
					Detail: schema.ImageURLDetailAuto,
				},
			}, nil
		}
		return schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeImageURL,
			ImageURL: &schema.ChatMessageImageURL{
				URL:    file.RelativePath,
				Detail: schema.ImageURLDetailAuto,
			},
		}, nil
	default:
		return schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeText,
			Text: fmt.Sprintf("[File: %s]", file.FileName),
		}, nil
	}
}

// buildImageInputPart 构建图片输入部分
func (b *MultimodalMessageBuilder) buildImageInputPart(file *MultimodalFile, useBase64 bool) (schema.MessageInputPart, error) {
	ext := filepath.Ext(file.FileName)
	mimeType := getMimeType(ext)

	if useBase64 {
		// 读取文件并转换为base64
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			return schema.MessageInputPart{}, errors.Newf(errors.ErrFileReadFailed, "failed to read image file %s: %v", file.FilePath, err)
		}

		base64Data := base64.StdEncoding.EncodeToString(data)

		// 同时保存文件路径和base64数据
		// URL字段存储文件路径（用于保存到数据库）
		// Base64Data字段存储base64（用于API调用）
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL:        &file.FilePath, // 保存文件路径
					Base64Data: &base64Data,    // 保存base64数据
					MIMEType:   mimeType,
				},
			},
		}, nil
	}

	// 使用URL方式
	return schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageInputImage{
			MessagePartCommon: schema.MessagePartCommon{
				URL:      &file.RelativePath,
				MIMEType: mimeType,
			},
		},
	}, nil
}

// buildAudioInputPart 构建音频输入部分
func (b *MultimodalMessageBuilder) buildAudioInputPart(file *MultimodalFile, useBase64 bool) (schema.MessageInputPart, error) {
	ext := filepath.Ext(file.FileName)
	mimeType := getMimeType(ext)

	if useBase64 {
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			return schema.MessageInputPart{}, errors.Newf(errors.ErrFileReadFailed, "failed to read audio file %s: %v", file.FilePath, err)
		}

		base64Data := base64.StdEncoding.EncodeToString(data)

		// 同时保存文件路径和base64数据
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeAudioURL,
			Audio: &schema.MessageInputAudio{
				MessagePartCommon: schema.MessagePartCommon{
					URL:        &file.FilePath, // 保存文件路径
					Base64Data: &base64Data,    // 保存base64数据
					MIMEType:   mimeType,
				},
			},
		}, nil
	}

	return schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeAudioURL,
		Audio: &schema.MessageInputAudio{
			MessagePartCommon: schema.MessagePartCommon{
				URL:      &file.RelativePath,
				MIMEType: mimeType,
			},
		},
	}, nil
}

// buildVideoInputPart 构建视频输入部分
func (b *MultimodalMessageBuilder) buildVideoInputPart(file *MultimodalFile, useBase64 bool) (schema.MessageInputPart, error) {
	ext := filepath.Ext(file.FileName)
	mimeType := getMimeType(ext)

	// 视频文件通常较大，只支持URL方式
	return schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeVideoURL,
		Video: &schema.MessageInputVideo{
			MessagePartCommon: schema.MessagePartCommon{
				URL:      &file.RelativePath,
				MIMEType: mimeType,
			},
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
