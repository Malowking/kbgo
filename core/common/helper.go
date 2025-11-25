package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Malowking/kbgo/core/indexer/file_store"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gfile"
)

func Of[T any](v T) *T {
	return &v
}

func IsURL(str string) bool {
	u, err := url.Parse(str)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

func RemoveDuplicates[T any, K comparable](slice []T, keyFunc func(T) K) []T {
	encountered := make(map[K]bool)
	var result []T

	for _, v := range slice {
		key := keyFunc(v)
		if !encountered[key] {
			encountered[key] = true
			result = append(result, v)
		}
	}

	return result
}

// HandleFileUpload 处理文件上传的通用逻辑
func HandleFileUpload(ctx context.Context, file *ghttp.UploadFile, urlStr string) (fileName string, fileExt string, fileSha256 string, tempFilePath string, err error) {
	if file != nil {
		fileName = file.Filename
		fileExt = filepath.Ext(fileName)
	} else if urlStr != "" {
		fileName = file_store.GetFileNameFromURL(urlStr)
		fileExt = filepath.Ext(fileName)
	} else {
		err = fmt.Errorf("file or url is required")
		return
	}

	// 根据文件类型确定临时存储目录
	tempUploadDir := file_store.GetUploadDirByFileType(fileExt)

	// 确保目录存在
	if !gfile.Exists(tempUploadDir) {
		err = gfile.Mkdir(tempUploadDir)
		if err != nil {
			return
		}
	}

	// 生成临时文件路径
	tempFilePath = filepath.Join(tempUploadDir, fileName)

	// 保存文件到临时位置
	if file != nil {
		_, err = file.Save(tempUploadDir)
		if err != nil {
			return
		}
	} else {
		// 下载URL文件到临时位置
		err = downloadURLFile(urlStr, tempUploadDir)
		if err != nil {
			return
		}
	}

	// 计算文件 SHA256 以确保唯一性
	fileSha256, err = calculateLocalFileSHA256(tempFilePath)
	if err != nil {
		return
	}

	return
}

// downloadURLFile 下载URL文件到本地
func downloadURLFile(url, localPath string) error {
	// 创建文件
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("Failed to create file: %w", err)
	}
	defer file.Close()

	// 下载文件
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to download file, status code: %d", resp.StatusCode)
	}

	// 保存文件内容
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to save file: %w", err)
	}

	return nil
}

// calculateLocalFileSHA256 计算本地文件的SHA256
func calculateLocalFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("Failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
