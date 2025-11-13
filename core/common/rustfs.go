package common

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/minio/minio-go/v7"
)

// UploadToRustFS 支持上传本地文件（multipart）或远程URL文件到 RustFS。
// file: *multipart.FileHeader 或 nil
// url: string 或空字符串
func UploadToRustFS(ctx context.Context, client *minio.Client, bucketName string, knowledgeId string, file interface{}, url string) (minio.UploadInfo, error) {
	var (
		reader      io.ReadCloser
		size        int64
		objectName  string
		contentType string
	)

	// 本地文件上传
	if f, ok := file.(*multipart.FileHeader); ok && f != nil {
		objectName = knowledgeId + "/" + f.Filename
		r, err := f.Open()
		if err != nil {
			return minio.UploadInfo{}, fmt.Errorf("open local file failed: %w", err)
		}
		reader = r
		size = f.Size
		if f.Header != nil {
			if vals, ok := f.Header["Content-Type"]; ok && len(vals) > 0 {
				contentType = vals[0]
			}
		}
	}

	// URL 文件上传
	if url != "" {
		resp, err := http.Get(url)
		if err != nil {
			return minio.UploadInfo{}, fmt.Errorf("download url failed: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return minio.UploadInfo{}, fmt.Errorf("url returned status: %s", resp.Status)
		}
		reader = resp.Body
		size = resp.ContentLength
		contentType = resp.Header.Get("Content-Type")
		objectName = knowledgeId + "/" + getFileNameFromURL(url)
	}

	if reader == nil {
		return minio.UploadInfo{}, fmt.Errorf("no file or url provided")
	}
	defer func() {
		_ = reader.Close()
	}()

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	info, err := client.PutObject(ctx, bucketName, objectName, reader, size,
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("upload failed: %w", err)
	}

	return info, nil
}

// 辅助函数：从URL中提取文件名
func getFileNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	name := parts[len(parts)-1]
	if name == "" {
		name = "unknown_file"
	}
	return name
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

// CheckObjectExists 检查指定的对象是否存在
func CheckObjectExists(ctx context.Context, client *minio.Client, bucketName, objectName string) (bool, error) {
	objects, err := ListObjects(ctx, client, bucketName, true)
	if err != nil {
		return false, err
	}

	for _, obj := range objects {
		if obj.Key == objectName {
			return true, nil
		}
	}

	return false, nil
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

// DownloadFileToPath 从 bucket 下载文件到指定的本地路径（公共函数）
func DownloadFileToPath(ctx context.Context, client *minio.Client, bucketName, objectName, filePath string) error {
	return DownloadFile(ctx, client, bucketName, objectName, filePath)
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

// GetObjectInfo 获取对象的详细信息
func GetObjectInfo(ctx context.Context, client *minio.Client, bucketName, objectName string) (minio.ObjectInfo, error) {
	objInfo, err := client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return minio.ObjectInfo{}, fmt.Errorf("failed to get object info: %w", err)
	}

	return objInfo, nil
}

// GetObject 获取对象的内容作为io.Reader
func GetObject(ctx context.Context, client *minio.Client, bucketName, objectName string) (io.ReadCloser, error) {
	reader, err := client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	return reader, nil
}

// ReadRustFSObject 读取RustFS对象内容并返回字节数据
func ReadRustFSObject(ctx context.Context, client *minio.Client, bucketName, objectName string) ([]byte, error) {
	reader, err := GetObject(ctx, client, bucketName, objectName)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}
