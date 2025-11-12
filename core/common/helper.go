package common

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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

// CalculateFileSHA256 计算上传文件的SHA256哈希值
func CalculateFileSHA256(file *multipart.FileHeader) (string, error) {
	f, err := file.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CalculateURLFileSHA256 计算 URL 文件的 SHA256 哈希值
func CalculateURLFileSHA256(fileURL string) (string, error) {
	// 下载文件内容
	resp, err := http.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("failed to download URL file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("URL returned status: %s", resp.Status)
	}

	// 计算 SHA256
	hash := sha256.New()
	if _, err := io.Copy(hash, resp.Body); err != nil {
		return "", fmt.Errorf("failed to calculate SHA256: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
