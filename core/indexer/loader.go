package indexer

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Malowking/kbgo/core/common"
	"github.com/cloudwego/eino-ext/components/document/loader/file"
	document_url "github.com/cloudwego/eino-ext/components/document/loader/url"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/minio/minio-go/v7"
)

// Loader component initialization function of node 'Loader1' in graph 'rag'
func Loader(ctx context.Context, client *minio.Client, bucketName string) (ldr document.Loader, err error) {
	mldr := &multiLoader{
		rustfsBucketName: bucketName,
		rustfsClient:     client,
	}

	parserInstance, err := newParser(ctx)
	if err != nil {
		return nil, err
	}

	fldr, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{
		UseNameAsID: false,
		Parser:      parserInstance,
	})
	if err != nil {
		return nil, err
	}
	mldr.fileLoader = fldr

	uldr, err := document_url.NewLoader(ctx, &document_url.LoaderConfig{})
	if err != nil {
		return nil, err
	}
	mldr.urlLoader = uldr

	// 设置 parser
	mldr.parser = parserInstance

	return mldr, nil
}

type multiLoader struct {
	fileLoader       document.Loader
	urlLoader        document.Loader
	rustfsClient     *minio.Client
	rustfsBucketName string
	parser           parser.Parser
}

func (x *multiLoader) Load(ctx context.Context, src document.Source, opts ...document.LoaderOption) ([]*schema.Document, error) {
	if common.IsURL(src.URI) {
		// 检查是否是 rustfs 协议
		if isRustFSURL(src.URI) {
			return x.loadRustFSObject(ctx, src)
		}
		return x.urlLoader.Load(ctx, src, opts...)
	}
	return x.fileLoader.Load(ctx, src, opts...)
}

// isRustFSURL 检查 URI 是否是 rustfs 协议
func isRustFSURL(uri string) bool {
	return len(uri) >= 6 && strings.HasPrefix(uri, "rustfs://")
}

// loadRustFSObject 从 rustfs 加载文档
func (x *multiLoader) loadRustFSObject(ctx context.Context, src document.Source) ([]*schema.Document, error) {
	// 解析 rustfs URL
	u, err := url.Parse(src.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rustfs URL: %w", err)
	}

	// 验证协议
	if u.Scheme != "rustfs" {
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	// 获取对象名称（路径）
	objectName := strings.TrimPrefix(u.Path, "/")
	if objectName == "" {
		return nil, fmt.Errorf("empty object name in URL: %s", src.URI)
	}

	// 从 RustFS 读取对象内容
	content, err := common.ReadRustFSObject(ctx, x.rustfsClient, x.rustfsBucketName, objectName)
	if err != nil {
		return nil, fmt.Errorf("failed to read object from rustfs: %w", err)
	}

	// 使用解析器解析内容
	reader := bytes.NewReader(content)
	docs, err := x.parser.Parse(ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document content: %w", err)
	}

	// 设置文档元数据
	for _, doc := range docs {
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["_source"] = src.URI
	}

	return docs, nil
}
