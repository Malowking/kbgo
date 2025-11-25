package file_store

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type RustfsConfig struct {
	Client     *minio.Client
	BucketName string
}

var rustfsConfig RustfsConfig

// InitRustFS 初始化 RustFS 存储
func InitRustFS(ctx context.Context, endpoint, accessKey, secretKey, bucketName string, ssl bool) error {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: ssl,
	})

	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// 设置全局配置
	rustfsConfig = RustfsConfig{
		Client:     client,
		BucketName: bucketName,
	}

	// CreateBucketIfNotExists 创建 bucket，如果已存在则跳过
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists: %w", err)
	}

	if exists {
		g.Log().Printf(ctx, "Bucket '%s' already exists, skipping creation.", bucketName)
		return nil
	}

	err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: ""})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	g.Log().Printf(ctx, "Created bucket '%s'", bucketName)
	return nil
}

// GetRustfsConfig 获取RustFS配置
func GetRustfsConfig() *RustfsConfig {
	return &rustfsConfig
}

// GetUploadDirByFileType 根据文件扩展名获取对应的上传目录
func GetUploadDirByFileType(fileExt string) string {
	// 图片格式
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".svg": true, ".webp": true, ".ico": true,
	}

	// 视频格式
	videoExts := map[string]bool{
		".mp4": true, ".avi": true, ".mov": true, ".wmv": true,
		".flv": true, ".mkv": true, ".webm": true, ".m4v": true,
	}

	// 音频格式
	audioExts := map[string]bool{
		".mp3": true, ".wav": true, ".flac": true, ".aac": true,
		".ogg": true, ".wma": true, ".m4a": true, ".opus": true,
	}

	// 转换为小写进行比较
	ext := filepath.Ext(strings.ToLower(fileExt))
	if ext == "" {
		ext = "." + strings.ToLower(fileExt)
	}

	if imageExts[ext] {
		return "upload/image"
	} else if videoExts[ext] {
		return "upload/video"
	} else if audioExts[ext] {
		return "upload/audio"
	}

	// 默认返回 file 目录
	return "upload/file"
}

// UploadToRustFS 上传文件到 RustFS。
func UploadToRustFS(ctx context.Context, client *minio.Client, bucketName string, knowledgeId string, filePath string) (minio.UploadInfo, error) {
	var (
		reader      io.ReadCloser
		size        int64
		objectName  string
		contentType string
	)

	// 从文件路径获取文件信息
	file, err := os.Open(filePath)
	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("open local file failed: %w", err)
	}
	reader = file

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		_ = reader.Close()
		return minio.UploadInfo{}, fmt.Errorf("get file stat failed: %w", err)
	}
	size = stat.Size()

	// 获取文件名作为对象名
	filename := filepath.Base(filePath)
	objectName = "knowledge_base/" + knowledgeId + "/" + filename

	// 尝试检测内容类型
	// 读取文件开头一小段数据用于检测内容类型
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil && err != io.EOF {
		_ = reader.Close()
		return minio.UploadInfo{}, fmt.Errorf("read file header failed: %w", err)
	}

	// 重置文件指针到开头
	_, err = file.Seek(0, 0)
	if err != nil {
		_ = reader.Close()
		return minio.UploadInfo{}, fmt.Errorf("seek file to beginning failed: %w", err)
	}

	contentType = http.DetectContentType(buffer)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	info, err := client.PutObject(ctx, bucketName, objectName, reader, size,
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		_ = reader.Close()
		return minio.UploadInfo{}, fmt.Errorf("upload failed: %w", err)
	}

	_ = reader.Close()
	return info, nil
}

// GetFileNameFromURL 从URL中提取文件名（公开函数）
func GetFileNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	name := parts[len(parts)-1]
	if name == "" {
		name = "unknown_file"
	}
	return name
}

// getFileNameFromURL 从URL中提取文件名（内部函数）
func getFileNameFromURL(url string) string {
	return GetFileNameFromURL(url)
}

// ListObjects 列举 bucket 中的所有对象
func ListObjects(ctx context.Context, client *minio.Client, bucketName string, recursive bool) ([]minio.ObjectInfo, error) {
	var objects []minio.ObjectInfo

	objectCh := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: recursive,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("list error: %w", object.Err)
		}
		objects = append(objects, object)
		g.Log().Infof(ctx, "Found object: %s", object.Key)
	}

	return objects, nil
}

// DeleteObject 删除指定的对象
func DeleteObject(ctx context.Context, client *minio.Client, bucketName, objectName string) error {
	err := client.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	g.Log().Infof(ctx, "Deleted object '%s' from bucket '%s'", objectName, bucketName)
	return nil
}

// DownloadFile 从 bucket 下载文件到本地
func DownloadFile(ctx context.Context, client *minio.Client, bucketName, objectName, destFile string) error {
	err := client.FGetObject(ctx, bucketName, objectName, destFile, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	g.Log().Infof(ctx, "Downloaded '%s' from bucket '%s' to '%s'", objectName, bucketName, destFile)
	return nil
}
