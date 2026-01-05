package file_store

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Malowking/kbgo/core/errors"
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
		return errors.Newf(errors.ErrInternalError, "failed to create MinIO client: %v", err)
	}

	// 设置全局配置
	rustfsConfig = RustfsConfig{
		Client:     client,
		BucketName: bucketName,
	}

	// CreateBucketIfNotExists 创建 bucket，如果已存在则跳过
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return errors.Newf(errors.ErrInternalError, "failed to check if bucket exists: %v", err)
	}

	if exists {
		g.Log().Printf(ctx, "Bucket '%s' already exists, skipping creation.", bucketName)
		return nil
	}

	err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: ""})
	if err != nil {
		return errors.Newf(errors.ErrInternalError, "failed to create bucket: %v", err)
	}

	g.Log().Printf(ctx, "Created bucket '%s'", bucketName)
	return nil
}

// GetRustfsConfig 获取RustFS配置
func GetRustfsConfig() *RustfsConfig {
	return &rustfsConfig
}

// SaveFileToRustFS 保存文件到 RustFS 存储
func SaveFileToRustFS(ctx context.Context, client *minio.Client, bucketName string, knowledgeId string, fileName string, file io.ReadSeeker) (localPath string, rustfsKey string, err error) {
	// 第一步：先保存到本地 upload/knowledge_file/知识库id/文件名
	targetDir := filepath.Join("upload", "knowledge_file", knowledgeId)

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		g.Log().Errorf(ctx, "Failed to create directory %s: %v", targetDir, err)
		return "", "", errors.Newf(errors.ErrFileUploadFailed, "failed to create directory %s: %v", targetDir, err)
	}

	// 构建本地文件路径
	localPath = filepath.Join(targetDir, fileName)

	// 创建本地文件
	destFile, err := os.Create(localPath)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to create local file %s: %v", localPath, err)
		return "", "", errors.Newf(errors.ErrFileUploadFailed, "failed to create local file %s: %v", localPath, err)
	}
	defer destFile.Close()

	// 将上传的文件内容复制到本地文件
	_, err = io.Copy(destFile, file)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to write local file %s: %v", localPath, err)
		// 删除创建失败的文件
		_ = os.Remove(localPath)
		return "", "", errors.Newf(errors.ErrFileUploadFailed, "failed to write local file %s: %v", localPath, err)
	}

	g.Log().Infof(ctx, "File saved to local storage: %s", localPath)

	// 第二步：上传到 RustFS，路径为 bucketName/knowledge_file/知识库id/文件名
	rustfsKey = filepath.Join("knowledge_file", knowledgeId, fileName)

	// 重新打开本地文件用于上传
	uploadFile, err := os.Open(localPath)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to open local file for upload: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileReadFailed, "failed to open local file for upload: %v", err)
	}
	defer uploadFile.Close()

	// 获取文件大小
	stat, err := uploadFile.Stat()
	if err != nil {
		g.Log().Errorf(ctx, "Failed to get file stat: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileReadFailed, "failed to get file stat: %v", err)
	}
	fileSize := stat.Size()

	// 检测内容类型
	buffer := make([]byte, 512)
	_, err = uploadFile.Read(buffer)
	if err != nil && err != io.EOF {
		g.Log().Errorf(ctx, "Failed to read file header: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileReadFailed, "failed to read file header: %v", err)
	}

	// 重置文件指针到开头
	_, err = uploadFile.Seek(0, 0)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to seek file to beginning: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileReadFailed, "failed to seek file to beginning: %v", err)
	}

	contentType := http.DetectContentType(buffer)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 上传到 RustFS
	_, err = client.PutObject(ctx, bucketName, rustfsKey, uploadFile, fileSize,
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		g.Log().Errorf(ctx, "Failed to upload file to RustFS: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileUploadFailed, "failed to upload to RustFS: %v", err)
	}

	g.Log().Infof(ctx, "File uploaded to RustFS: bucket=%s, key=%s", bucketName, rustfsKey)
	return localPath, rustfsKey, nil
}

// SaveFileToRustFSNL2SQL 保存NL2SQL文件到RustFS存储
func SaveFileToRustFSNL2SQL(client *minio.Client, bucketName string, fileName string, file io.ReadSeeker) (localPath string, rustfsKey string, err error) {
	ctx := context.Background()

	// 第一步：保存到本地 upload/nl2sql/文件名
	targetDir := filepath.Join("upload", "nl2sql")

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		g.Log().Errorf(ctx, "Failed to create directory %s: %v", targetDir, err)
		return "", "", errors.Newf(errors.ErrFileUploadFailed, "failed to create directory %s: %v", targetDir, err)
	}

	// 构建本地文件路径
	localPath = filepath.Join(targetDir, fileName)

	// 创建本地文件
	destFile, err := os.Create(localPath)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to create local file %s: %v", localPath, err)
		return "", "", errors.Newf(errors.ErrFileUploadFailed, "failed to create local file %s: %v", localPath, err)
	}
	defer destFile.Close()

	// 将文件内容复制到本地文件
	_, err = io.Copy(destFile, file)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to write local file %s: %v", localPath, err)
		_ = os.Remove(localPath)
		return "", "", errors.Newf(errors.ErrFileUploadFailed, "failed to write local file %s: %v", localPath, err)
	}

	g.Log().Infof(ctx, "NL2SQL file saved to local storage: %s", localPath)

	// 第二步：上传到RustFS，路径为 bucketName/nl2sql/文件名
	rustfsKey = filepath.Join("nl2sql", fileName)

	// 重新打开本地文件用于上传
	uploadFile, err := os.Open(localPath)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to open local file for upload: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileReadFailed, "failed to open local file for upload: %v", err)
	}
	defer uploadFile.Close()

	// 获取文件大小
	stat, err := uploadFile.Stat()
	if err != nil {
		g.Log().Errorf(ctx, "Failed to get file stat: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileReadFailed, "failed to get file stat: %v", err)
	}
	fileSize := stat.Size()

	// 检测内容类型
	buffer := make([]byte, 512)
	_, err = uploadFile.Read(buffer)
	if err != nil && err != io.EOF {
		g.Log().Errorf(ctx, "Failed to read file header: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileReadFailed, "failed to read file header: %v", err)
	}

	// 重置文件指针到开头
	_, err = uploadFile.Seek(0, 0)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to seek file to beginning: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileReadFailed, "failed to seek file to beginning: %v", err)
	}

	contentType := http.DetectContentType(buffer)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 上传到RustFS
	_, err = client.PutObject(ctx, bucketName, rustfsKey, uploadFile, fileSize,
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		g.Log().Errorf(ctx, "Failed to upload NL2SQL file to RustFS: %v", err)
		return localPath, "", errors.Newf(errors.ErrFileUploadFailed, "failed to upload to RustFS: %v", err)
	}

	g.Log().Infof(ctx, "NL2SQL file uploaded to RustFS: bucket=%s, key=%s", bucketName, rustfsKey)
	return localPath, rustfsKey, nil
}

// GetFileNameFromURL 从URL中提取文件名
func GetFileNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	name := parts[len(parts)-1]
	if name == "" {
		name = "unknown_file"
	}
	return name
}

// DeleteObject 删除指定的对象
func DeleteObject(ctx context.Context, client *minio.Client, bucketName, objectName string) error {
	err := client.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return errors.Newf(errors.ErrFileDeleteFailed, "failed to delete object %s: %v", objectName, err)
	}
	g.Log().Infof(ctx, "Deleted object '%s' from bucket '%s'", objectName, bucketName)
	return nil
}

// DownloadFile 从 bucket 下载文件到本地
func DownloadFile(ctx context.Context, client *minio.Client, bucketName, objectName, destFile string) error {
	err := client.FGetObject(ctx, bucketName, objectName, destFile, minio.GetObjectOptions{})
	if err != nil {
		return errors.Newf(errors.ErrFileReadFailed, "failed to download file %s: %v", objectName, err)
	}
	g.Log().Infof(ctx, "Downloaded '%s' from bucket '%s' to '%s'", objectName, bucketName, destFile)
	return nil
}
