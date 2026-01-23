package common

// MimeTypeMap 统一的MIME类型映射表
var MimeTypeMap = map[string]string{
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
	".tif":  "image/tiff",

	// 音频格式
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".flac": "audio/flac",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".m4a":  "audio/mp4",
	".wma":  "audio/x-ms-wma",
	".opus": "audio/opus",

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
	".3gp":  "video/3gpp",
}

// GetMimeType 根据文件扩展名获取MIME类型
func GetMimeType(ext string) string {
	if mime, ok := MimeTypeMap[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// GetMimeTypeOrDefault 根据文件扩展名获取MIME类型
func GetMimeTypeOrDefault(ext string, defaultType string) string {
	if mime, ok := MimeTypeMap[ext]; ok {
		return mime
	}
	return defaultType
}

// IsImageMimeType 判断MIME类型是否为图片
func IsImageMimeType(mimeType string) bool {
	switch mimeType {
	case "image/jpeg", "image/png", "image/gif", "image/bmp", "image/webp",
		"image/svg+xml", "image/x-icon", "image/tiff":
		return true
	default:
		return false
	}
}

// IsAudioMimeType 判断MIME类型是否为音频
func IsAudioMimeType(mimeType string) bool {
	switch mimeType {
	case "audio/mpeg", "audio/wav", "audio/flac", "audio/aac", "audio/ogg",
		"audio/mp4", "audio/x-ms-wma", "audio/opus":
		return true
	default:
		return false
	}
}

// IsVideoMimeType 判断MIME类型是否为视频
func IsVideoMimeType(mimeType string) bool {
	switch mimeType {
	case "video/mp4", "video/x-msvideo", "video/x-matroska", "video/quicktime",
		"video/x-ms-wmv", "video/x-flv", "video/webm", "video/mpeg", "video/3gpp":
		return true
	default:
		return false
	}
}
