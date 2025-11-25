package file_store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogf/gf/v2/frame/g"
)

// LocalStorage 本地存储实现
type LocalStorage struct {
}

// NewLocalStorage 创建本地存储实例
func NewLocalStorage() *LocalStorage {
	return &LocalStorage{}
}

func InitKnowledgeFileDirectory() {

}

// SaveFile 保存文件到本地存储
func (l *LocalStorage) SaveFile(ctx context.Context, sourcePath, targetDir, filename string) (string, error) {
	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// 构建目标文件路径
	targetPath := filepath.Join(targetDir, filename)

	// 读取源文件
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to read source file %s: %w", sourcePath, err)
	}

	// 写入目标文件
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write target file %s: %w", targetPath, err)
	}

	g.Log().Debugf(ctx, "File saved to local storage: %s", targetPath)
	return targetPath, nil
}

// DeleteFile 从本地存储删除文件
func (l *LocalStorage) DeleteFile(ctx context.Context, filePath string) error {
	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	g.Log().Debugf(ctx, "File deleted from local storage: %s", filePath)
	return nil
}

// FileExists 检查文件是否存在于本地存储
func (l *LocalStorage) FileExists(ctx context.Context, filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}
