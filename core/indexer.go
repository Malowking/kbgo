package core

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/Malowking/kbgo/api/rag/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

type IndexReq struct {
	Docs           []*schema.Document
	KnowledgeId    string // 知识库ID
	CollectionName string // Milvus text collection 名称
	DocumentsId    string // 文档ID
	ChunkSize      int    // 文档分块大小
	OverlapSize    int    // 分块重叠大小
}

type IndexAsyncReq struct {
	Docs           []*schema.Document
	KnowledgeId    string // 知识库名称
	CollectionName string // Milvus collection 名称
	DocumentsId    string // 文档ID
	ChunkSize      int    // 文档分块大小
	OverlapSize    int    // 分块重叠大小
}

type IndexAsyncByDocsIDReq struct {
	DocsIDs        []string
	KnowledgeId    string // 知识库名称
	CollectionName string // Milvus text collection 名称
	//QACollectionName   string // Milvus QA collection 名称
	DocumentId string // 文档ID
}

// Index
// 这里处理文档的读取、分割、合并和存储
// 真正的embedding
func (x *Rag) Index(ctx context.Context, req *IndexReq) (ids []string, err error) {
	// 动态创建 text indexer，传递chunk参数
	idxer, err := x.BuildIndexer(ctx, req.CollectionName, req.ChunkSize, req.OverlapSize)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, common.KnowledgeId, req.KnowledgeId)
	ctx = context.WithValue(ctx, common.DocumentId, req.DocumentsId)
	ids, err = idxer.Invoke(ctx, req.Docs)
	if err != nil {
		return
	}
	go func() {
		// Milvus数据写入后立即可见（flush操作在indexer中已完成），所以不需要sleep
		ctxN := gctx.New()
		defer func() {
			if e := recover(); e != nil {
				g.Log().Errorf(ctxN, "recover indexAsyncByDocsID failed, err=%v", e)
			}
		}()
		_, err = x.indexAsyncByDocsID(ctxN, &IndexAsyncByDocsIDReq{
			DocsIDs:        ids,
			KnowledgeId:    req.KnowledgeId,
			CollectionName: req.CollectionName,
			//QACollectionName:   "", // Will be set in indexAsyncByDocsID
			DocumentId: req.DocumentsId,
		})
		if err != nil {
			g.Log().Errorf(ctxN, "indexAsyncByDocsID failed, err=%v", err)
		}
	}()
	return
}

// TODO 添加QA功能
// IndexAsync
// 通过 schema.Document 异步 生成QA&embedding
func (x *Rag) IndexAsync(ctx context.Context, req *IndexAsyncReq) (ids []string, err error) {
	// 动态创建indexer
	idxerAsync, err := x.BuildIndexer(ctx, req.CollectionName, req.ChunkSize, req.OverlapSize)
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, common.KnowledgeId, req.KnowledgeId)
	ctx = context.WithValue(ctx, common.DocumentId, req.DocumentsId)
	ids, err = idxerAsync.Invoke(ctx, req.Docs)
	if err != nil {
		return
	}

	return
}

// 通过docIDs 异步 生成QA&embedding
// 这个方法不用暴露出去
func (x *Rag) indexAsyncByDocsID(ctx context.Context, req *IndexAsyncByDocsIDReq) (ids []string, err error) {
	// 从 Milvus 查询数据，根据 ID 列表获取文档
	client := x.Client
	filter := fmt.Sprintf("id in [%s]", joinIDs(req.DocsIDs))

	queryOpt := milvusclient.NewQueryOption(req.CollectionName).
		WithFilter(filter).
		WithOutputFields("id", "text", "document_id", "metadata")

	queryResult, err := client.Query(ctx, queryOpt)
	if err != nil {
		g.Log().Errorf(ctx, "milvus query failed, err=%v", err)
		return nil, err
	}

	var docs []*schema.Document
	var chunks []entity.KnowledgeChunks

	// 解析查询结果 - Milvus v2 SDK 的结果是 column 数据
	resultCount := queryResult.ResultCount
	if resultCount == 0 {
		g.Log().Warningf(ctx, "indexAsyncByDocsID no docs found in Milvus, DocsIDs=%v", req.DocsIDs)
		return nil, fmt.Errorf("no docs found for DocsIDs: %v", req.DocsIDs)
	}

	// 获取各个字段的 column
	fields := queryResult.Fields

	// 辅助函数：根据名称获取 column
	getColumn := func(name string) column.Column {
		for _, col := range fields {
			if col.Name() == name {
				return col
			}
		}
		return nil
	}

	idCol := getColumn("id")
	textCol := getColumn("text")
	metadataCol := getColumn("metadata")

	if idCol == nil || textCol == nil {
		g.Log().Errorf(ctx, "required columns not found in query result")
		return nil, fmt.Errorf("required columns not found")
	}

	for i := 0; i < resultCount; i++ {
		doc := &schema.Document{}

		// 获取 ID
		if idVal, err := idCol.GetAsString(i); err == nil {
			doc.ID = idVal
		} else {
			g.Log().Errorf(ctx, "failed to get id at index %d: %v", i, err)
			continue
		}

		// 获取文本内容
		if textVal, err := textCol.GetAsString(i); err == nil {
			doc.Content = textVal
		} else {
			g.Log().Errorf(ctx, "failed to get text at index %d: %v", i, err)
		}

		// 获取元数据
		if metadataCol != nil {
			if metadataVal, err := metadataCol.Get(i); err == nil {
				switch v := metadataVal.(type) {
				case string:
					var metadata map[string]any
					if err := sonic.Unmarshal([]byte(v), &metadata); err == nil {
						doc.MetaData = metadata
					}
				case []byte:
					var metadata map[string]any
					if err := sonic.Unmarshal(v, &metadata); err == nil {
						doc.MetaData = metadata
					}
				}
			}
		}

		// 如果 metadata 为空，初始化为空 map
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]any)
		}

		docs = append(docs, doc)

		// 准备保存到数据库的 chunks 数据
		ext, err := sonic.Marshal(doc.MetaData)
		if err != nil {
			g.Log().Errorf(ctx, "sonic.Marshal failed, err=%v", err)
			continue
		}
		chunks = append(chunks, entity.KnowledgeChunks{
			Id:             doc.ID,
			KnowledgeDocId: req.DocumentId,
			Content:        doc.Content,
			Ext:            string(ext),
			CollectionName: req.CollectionName,
		})
	}

	if len(docs) == 0 {
		g.Log().Warningf(ctx, "indexAsyncByDocsID no valid docs after parsing")
		return nil, fmt.Errorf("no valid docs found after parsing")
	}

	// 保存 chunks 数据到数据库
	if err = knowledge.SaveChunksData(ctx, req.DocumentId, chunks); err != nil {
		// 这里不返回err，不影响用户使用
		g.Log().Errorf(ctx, "indexAsyncByDocsID insert chunks failed, err=%v", err)
	}

	err = knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusActive))
	if err != nil {
		g.Log().Errorf(ctx, "update documents status failed, err=%v", err)
	}
	// TODO 添加QA功能
	// 获取 QA collection name
	// 如果没有传入，则根据 text collection name 生成
	//qaCollectionName := req.QACollectionName
	//if qaCollectionName == "" {
	//	// text_xxx -> qa_xxx
	//	if len(req.TextCollectionName) > 5 && req.TextCollectionName[:5] == "text_" {
	//		qaCollectionName = "qa_" + req.TextCollectionName[5:]
	//	} else {
	//		g.Log().Errorf(ctx, "invalid TextCollectionName: %s", req.TextCollectionName)
	//		return nil, fmt.Errorf("invalid TextCollectionName: %s", req.TextCollectionName)
	//	}
	//}
	//
	//asyncReq := &IndexAsyncReq{
	//	Docs:             docs,
	//	KnowledgeId:      req.KnowledgeId,
	//	QACollectionName: qaCollectionName,
	//	DocumentsId:      req.DocumentsId,
	//}
	//ids, err = x.IndexAsync(ctx, asyncReq)
	//if err != nil {
	//	return
	//}
	//_ = knowledge.UpdateDocumentsStatus(ctx, req.DocumentsId, int(v1.StatusActive))
	return
}

// joinIDs 将 ID 列表转换为 Milvus filter 表达式格式
func joinIDs(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf(`"%s"`, id)
	}
	return strings.Join(quoted, ",")
}

func (x *Rag) DeleteDocument(ctx context.Context, collectionName string, documentID string) error {
	return common.DeleteMilvusDocument(ctx, x.Client, collectionName, documentID)
}

func (x *Rag) DeleteChunk(ctx context.Context, collectionName string, chunkID string) error {
	return common.DeleteMilvusChunk(ctx, x.Client, collectionName, chunkID)
}
