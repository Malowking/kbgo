package core

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type RustfsConfig struct {
	Client     *minio.Client
	BucketName string
}

var rustfsConfig RustfsConfig

func init() {
	ctx := gctx.New()
	// 检查rustfs配置是否存在
	rustfsEndpoint := g.Cfg().MustGet(ctx, "rustfs.endpoint", "").String()
	if rustfsEndpoint == "" {
		g.Log().Fatalf(ctx, "RustFs configuration is missing: rustfs.endpoint is required")
		return
	}
	rustfsAccessKey := g.Cfg().MustGet(ctx, "rustfs.accessKey").String()
	rustfsSecretKey := g.Cfg().MustGet(ctx, "rustfs.secretKey").String()
	rustfsBucketName := g.Cfg().MustGet(ctx, "rustfs.bucketName").String()
	rustfsSsl := g.Cfg().MustGet(ctx, "rustfs.ssl", false).Bool()

	client, err := minio.New(rustfsEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(rustfsAccessKey, rustfsSecretKey, ""),
		Secure: rustfsSsl,
	})

	if err != nil {
		g.Log().Fatalf(ctx, "failed to create MinIO client: %w", err)
		return
	}

	// 设置全局配置（无论 bucket 是否已存在）
	rustfsConfig = RustfsConfig{
		Client:     client,
		BucketName: rustfsBucketName,
	}

	// CreateBucketIfNotExists 创建 bucket，如果已存在则跳过
	exists, err := client.BucketExists(ctx, rustfsBucketName)
	if err != nil {
		g.Log().Fatalf(ctx, "failed to check if bucket exists: %w", err)
		return
	}

	if exists {
		g.Log().Printf(ctx, "Bucket '%s' already exists, skipping creation.", rustfsBucketName)
		return
	}

	err = client.MakeBucket(ctx, rustfsBucketName, minio.MakeBucketOptions{Region: ""})
	if err != nil {
		g.Log().Printf(ctx, "failed to create bucket: %w", err)
		return
	}
	g.Log().Printf(ctx, "Created bucket '%s'", rustfsBucketName)
}

func GetRustfsConfig() *RustfsConfig {
	return &rustfsConfig
}
