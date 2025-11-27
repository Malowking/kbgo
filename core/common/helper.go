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

	"github.com/Malowking/kbgo/core/file_store"
	"github.com/gogf/gf/v2/net/ghttp"
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
// 返回文件名、文件扩展名、文件SHA256和文件读取器
func HandleFileUpload(ctx context.Context, file *ghttp.UploadFile, urlStr string) (fileName string, fileExt string, fileSha256 string, fileReader io.ReadSeeker, err error) {
	if file != nil {
		fileName = file.Filename
		fileExt = filepath.Ext(fileName)

		// 打开上传的文件
		fileReader, err = file.Open()
		if err != nil {
			err = fmt.Errorf("failed to open uploaded file: %w", err)
			return
		}

		// 计算 SHA256
		hash := sha256.New()
		if _, err = io.Copy(hash, fileReader); err != nil {
			return
		}
		fileSha256 = hex.EncodeToString(hash.Sum(nil))

		// 重置文件指针到开头
		_, err = fileReader.Seek(0, io.SeekStart)
		if err != nil {
			err = fmt.Errorf("failed to seek file: %w", err)
			return
		}

	} else if urlStr != "" {
		fileName = file_store.GetFileNameFromURL(urlStr)
		fileExt = filepath.Ext(fileName)

		// 下载URL文件到临时位置
		tempFilePath := filepath.Join(os.TempDir(), fileName)
		err = downloadURLFile(urlStr, tempFilePath)
		if err != nil {
			return
		}

		// 计算文件 SHA256
		fileSha256, err = calculateLocalFileSHA256(tempFilePath)
		if err != nil {
			_ = os.Remove(tempFilePath)
			return
		}

		// 打开文件作为 ReadSeeker
		var f *os.File
		f, err = os.Open(tempFilePath)
		if err != nil {
			_ = os.Remove(tempFilePath)
			return
		}
		fileReader = f

	} else {
		err = fmt.Errorf("file or url is required")
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
