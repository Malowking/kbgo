package file_store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gogf/gf/v2/os/gctx"

	"github.com/gogf/gf/v2/frame/g"
)

// StorageType 存储类型
type StorageType string

const (
	StorageTypeRustFS StorageType = "rustfs"
	StorageTypeLocal  StorageType = "local"
)

var storageType StorageType

// InitUploadDirectories 初始化 upload 目录结构到项目根目录
func InitUploadDirectories() {
	ctx := gctx.New()

	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		g.Log().Warningf(ctx, "Failed to get working directory: %v", err)
		return
	}

	// 查找项目根目录
	projectRoot := wd
	for !strings.HasSuffix(projectRoot, "kbgo") && projectRoot != "/" {
		projectRoot = filepath.Dir(projectRoot)
	}

	// 如果找不到 kbgo 目录，则使用当前工作目录
	if projectRoot == "/" {
		projectRoot = wd
	}

	// 定义需要创建的目录（基于项目根目录）
	uploadDirs := []string{
		filepath.Join(projectRoot, "upload"),
		filepath.Join(projectRoot, "upload/image"),
		filepath.Join(projectRoot, "upload/video"),
		filepath.Join(projectRoot, "upload/audio"),
		filepath.Join(projectRoot, "upload/file"),
		filepath.Join(projectRoot, "upload/knowledge_file"),
		filepath.Join(projectRoot, "upload/nl2sql"),
		filepath.Join(projectRoot, "upload/export"),
	}

	// 创建所有目录
	for _, dir := range uploadDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			g.Log().Warningf(ctx, "Failed to create directory %s: %v", dir, err)
		}
	}
}

// SetStorageType 设置存储类型
func SetStorageType(storageTypeVal StorageType) {
	storageType = storageTypeVal
}

// GetStorageType 获取存储类型
func GetStorageType() StorageType {
	return storageType
}
