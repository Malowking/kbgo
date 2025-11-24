package indexer

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestTransformerWithMarkdown(t *testing.T) {
	ctx := context.Background()

	// 创建一个包含Markdown内容的测试文档
	docs := []*schema.Document{
		{
			Content: "# 标题1\n这是标题1下的内容。\n## 标题2\n这是标题2下的内容。\n### 标题3\n这是标题3下的内容。",
			MetaData: map[string]interface{}{
				"_extension": ".md",
			},
		},
	}

	// 创建Transformer
	transformer, err := NewTransformer(ctx, 100, 20, "")
	assert.NoError(t, err)
	assert.NotNil(t, transformer)

	// 执行转换
	transformedDocs, err := transformer.Transform(ctx, docs)
	assert.NoError(t, err)
	assert.NotNil(t, transformedDocs)

	// 验证结果
	t.Logf("Transformed %d documents", len(transformedDocs))
	for i, doc := range transformedDocs {
		t.Logf("Document %d: %s", i, doc.Content)
		if doc.MetaData != nil {
			for k, v := range doc.MetaData {
				t.Logf("  Metadata %s: %v", k, v)
			}
		}
	}
}

func TestTransformerWithPlainText(t *testing.T) {
	ctx := context.Background()

	// 创建一个普通文本内容的测试文档
	docs := []*schema.Document{
		{
			Content: "这是第一段内容。这是第二句话，用来测试分割效果。这是第三句话，看看是否会被正确分割。最后是第四句话。",
			MetaData: map[string]interface{}{
				"_extension": ".txt",
			},
		},
	}

	// 创建Transformer
	transformer, err := NewTransformer(ctx, 30, 5, "")
	assert.NoError(t, err)
	assert.NotNil(t, transformer)

	// 执行转换
	transformedDocs, err := transformer.Transform(ctx, docs)
	assert.NoError(t, err)
	assert.NotNil(t, transformedDocs)

	// 验证结果
	t.Logf("Transformed %d documents", len(transformedDocs))
	for i, doc := range transformedDocs {
		t.Logf("Document %d: %s", i, doc.Content)
	}
}

func TestTransformerWithCustomSeparator(t *testing.T) {
	ctx := context.Background()

	// 创建一个使用自定义分隔符的测试文档
	docs := []*schema.Document{
		{
			Content: "第一部分|第二部分|第三部分|第四部分",
			MetaData: map[string]interface{}{
				"_extension": ".txt",
			},
		},
	}

	// 创建使用自定义分隔符的Transformer
	transformer, err := NewTransformer(ctx, 3, 1, "|")
	assert.NoError(t, err)
	assert.NotNil(t, transformer)

	// 执行转换
	transformedDocs, err := transformer.Transform(ctx, docs)
	assert.NoError(t, err)
	assert.NotNil(t, transformedDocs)

	// 验证结果
	assert.GreaterOrEqual(t, len(transformedDocs), 3, "Should have at least 3 documents after splitting")

	t.Logf("Transformed %d documents", len(transformedDocs))
	for i, doc := range transformedDocs {
		t.Logf("Document %d: %s", i, doc.Content)
	}
}

func TestTransformerWithEmptyDocument(t *testing.T) {
	ctx := context.Background()

	// 创建一个空文档
	docs := []*schema.Document{
		{
			Content: "",
			MetaData: map[string]interface{}{
				"_extension": ".txt",
			},
		},
	}

	// 创建Transformer
	transformer, err := NewTransformer(ctx, 100, 20, "")
	assert.NoError(t, err)
	assert.NotNil(t, transformer)

	// 执行转换
	transformedDocs, err := transformer.Transform(ctx, docs)
	assert.NoError(t, err)
	assert.NotNil(t, transformedDocs)

	// 验证结果
	assert.Equal(t, 0, len(transformedDocs), "Empty documents should be filtered out")
}

func TestTransformerWithLargeDocument(t *testing.T) {
	ctx := context.Background()

	// 创建一个大文档
	content := ""
	for i := 0; i < 100; i++ {
		content += "这是第" + string(rune(i+'0')) + "行内容，用来测试大文档的分割效果。"
	}

	docs := []*schema.Document{
		{
			Content: content,
			MetaData: map[string]interface{}{
				"_extension": ".txt",
			},
		},
	}

	// 创建Transformer
	transformer, err := NewTransformer(ctx, 50, 10, "")
	assert.NoError(t, err)
	assert.NotNil(t, transformer)

	// 执行转换
	transformedDocs, err := transformer.Transform(ctx, docs)
	assert.NoError(t, err)
	assert.NotNil(t, transformedDocs)

	// 验证结果
	assert.Greater(t, len(transformedDocs), 1, "Should have multiple documents after splitting large document")

	t.Logf("Transformed %d documents from large document", len(transformedDocs))
	for i, doc := range transformedDocs {
		t.Logf("Document %d length: %d", i, len(doc.Content))
	}
}
