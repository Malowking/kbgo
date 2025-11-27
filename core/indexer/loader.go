package indexer

import (
	"context"

	"github.com/Malowking/kbgo/core/common"
	"github.com/cloudwego/eino-ext/components/document/loader/file"
	document_url "github.com/cloudwego/eino-ext/components/document/loader/url"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

// Loader component initialization function of node 'Loader1' in graph 'retriever'
func Loader(ctx context.Context) (ldr document.Loader, err error) {
	mldr := &multiLoader{}

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
	fileLoader document.Loader
	urlLoader  document.Loader
	parser     parser.Parser
}

func (x *multiLoader) Load(ctx context.Context, src document.Source, opts ...document.LoaderOption) ([]*schema.Document, error) {
	if common.IsURL(src.URI) {
		// 对于 URL，使用 URL 加载器
		return x.urlLoader.Load(ctx, src, opts...)
	}
	// 对于本地文件，使用文件加载器
	return x.fileLoader.Load(ctx, src, opts...)
}
