package file_store

import (
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/gogf/gf/v2/frame/g"
)

// InitStorage 初始化存储系统
func InitStorage() {
	ctx := gctx.New()

	// 获取存储类型配置，默认为 local
	storageTypeStr := g.Cfg().MustGet(ctx, "storage.type", "local").String()

	// 根据配置决定存储类型
	switch storageTypeStr {
	case "local":
		SetStorageType(StorageTypeLocal)
		g.Log().Infof(ctx, "Using local storage as configured")
		// 初始化 upload 目录结构
		InitUploadDirectories()
		return
	case "rustfs":
		// 检查rustfs配置是否存在
		rustfsEndpoint := g.Cfg().MustGet(ctx, "rustfs.endpoint", "").String()
		if rustfsEndpoint == "" {
			// 如果没有配置rustfs，使用本地存储
			SetStorageType(StorageTypeLocal)
			g.Log().Infof(ctx, "RustFS not configured, using local storage")
			// 初始化 upload 目录结构
			InitUploadDirectories()
			return
		}

		SetStorageType(StorageTypeRustFS)
		rustfsAccessKey := g.Cfg().MustGet(ctx, "rustfs.accessKey").String()
		rustfsSecretKey := g.Cfg().MustGet(ctx, "rustfs.secretKey").String()
		rustfsBucketName := g.Cfg().MustGet(ctx, "rustfs.bucketName").String()
		rustfsSsl := g.Cfg().MustGet(ctx, "rustfs.ssl", false).Bool()

		err := InitRustFS(ctx, rustfsEndpoint, rustfsAccessKey, rustfsSecretKey, rustfsBucketName, rustfsSsl)
		if err != nil {
			g.Log().Fatalf(ctx, "failed to initialize RustFS: %v", err)
			return
		}

		g.Log().Infof(ctx, "Using RustFS storage as configured")
		// 初始化 upload 目录结构
		InitUploadDirectories()
		return
	default:
		// 默认使用本地存储
		SetStorageType(StorageTypeLocal)
		g.Log().Infof(ctx, "Using local storage as default")
		// 初始化 upload 目录结构
		InitUploadDirectories()
		return
	}
}
