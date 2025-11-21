package core

import (
	"context"
	"fmt"
	"os"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/indexer"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// 批量索引请求参数
type BatchIndexReq struct {
	DocumentIds []string // 文档ID列表
	ChunkSize   int      // 文档分块大小
	OverlapSize int      // 分块重叠大小
	Separator   string   // 自定义分隔符
}

// 统一索引请求参数
type IndexReq struct {
	DocumentId  string // 文档ID
	ChunkSize   int    // 文档分块大小
	OverlapSize int    // 分块重叠大小
	Separator   string // 自定义分隔符
}

// BatchDocumentIndex 批量处理文档索引（异步操作）
func (x *Rag) BatchDocumentIndex(ctx context.Context, req *BatchIndexReq) error {
	g.Log().Infof(ctx, "开始批量文档索引，文档数量: %d", len(req.DocumentIds))

	// 异步处理每个文档
	for _, documentId := range req.DocumentIds {
		go func(docId string) {
			defer func() {
				if e := recover(); e != nil {
					g.Log().Errorf(ctx, "文档索引异, documentId=%s, err=%v", docId, e)
					knowledge.UpdateDocumentsStatus(ctx, docId, int(v1.StatusFailed))
				}
			}()

			indexReq := &IndexReq{
				DocumentId:  docId,
				ChunkSize:   req.ChunkSize,
				OverlapSize: req.OverlapSize,
				Separator:   req.Separator,
			}

			err := x.DocumentIndex(ctx, indexReq)
			if err != nil {
				g.Log().Errorf(ctx, "文档索引失败, documentId=%s, err=%v", docId, err)
			} else {
				g.Log().Infof(ctx, "文档索引成功, documentId=%s", docId)
			}
		}(documentId)
	}

	return nil
}

// UnifiedDocumentIndex 统一处理文档索引（包含状态检查和存储类型处理）
func (x *Rag) DocumentIndex(ctx context.Context, req *IndexReq) error {
	// 1. 获取文档信息
	doc, err := knowledge.GetDocumentById(ctx, req.DocumentId)
	if err != nil {
		g.Log().Errorf(ctx, "获取文档信息失败, documentId=%s, err=%v", req.DocumentId, err)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		return err
	}

	// 2. 检查文档状态，如果已经是成功状态则跳过
	if doc.Status == int(v1.StatusActive) {
		g.Log().Infof(ctx, "文档已成功索引，跳过处理, documentId=%s", req.DocumentId)
		return nil
	}

	// 3. 首先删除该文档的所有相关数据
	err = knowledge.DeleteDocumentDataOnly(ctx, req.DocumentId, x.Client)
	if err != nil {
		g.Log().Errorf(ctx, "删除文档旧数据失败, documentId=%s, err=%v", req.DocumentId, err)
		_ = knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		return err
	}

	// 4. 获取存储类型配置
	uploadDir := common.GetUploadDirByFileType(doc.FileExtension)

	// 5. 文件路径处理（只从upload目录读取文件）
	localFilePath := gfile.Join(uploadDir, doc.FileName)

	// 6. 检查upload目录下文件是否存在
	if localFilePath == "" || !fileExists(localFilePath) {
		err = fmt.Errorf("upload目录下文件不存在, path=%s", localFilePath)
		g.Log().Errorf(ctx, "upload目录下文件不存在, documentId=%s, path=%s", req.DocumentId, localFilePath)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		return err
	}

	// 7. 创建Loader
	loader, err := indexer.Loader(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "创建Loader失败, documentId=%s, err=%v", req.DocumentId, err)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		// 删除upload目录下的文件
		os.Remove(localFilePath)
		return err
	}

	// 8. 加载文档
	docs, err := loader.Load(ctx, document.Source{URI: localFilePath})
	if err != nil {
		g.Log().Errorf(ctx, "加载文档失败, documentId=%s, err=%v", req.DocumentId, err)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		// 删除upload目录下的文件
		os.Remove(localFilePath)
		return err
	}

	// 9. 创建Transformer进行切分
	var transformer document.Transformer
	if req.Separator != "" {
		// 使用自定义分隔符
		transformer, err = indexer.SeparatorSplitter(ctx, req.Separator, req.ChunkSize, req.OverlapSize)
	} else {
		// 使用默认分隔符
		transformer, err = indexer.NewTransformer(ctx, req.ChunkSize, req.OverlapSize)
	}
	if err != nil {
		g.Log().Errorf(ctx, "创建Transformer失败, documentId=%s, err=%v", req.DocumentId, err)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		// 删除upload目录下的文件
		os.Remove(localFilePath)
		return err
	}

	// 10. 切分文档
	chunks, err := transformer.Transform(ctx, docs)
	if err != nil {
		g.Log().Errorf(ctx, "文档切分失败, documentId=%s, err=%v", req.DocumentId, err)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		// 删除upload目录下的文件
		os.Remove(localFilePath)
		return err
	}

	g.Log().Infof(ctx, "文档切分完成, documentId=%s, chunk数量=%d", req.DocumentId, len(chunks))

	// 11. 保存chunks到数据库
	err = x.saveChunksToDatabase(ctx, req.DocumentId, doc.CollectionName, chunks)
	if err != nil {
		g.Log().Errorf(ctx, "保存chunks到数据库失败, documentId=%s, err=%v", req.DocumentId, err)
		// 切分成功但向量化失败，设置状态为StatusIndexing
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusIndexing))
		// 删除upload目录下的文件
		os.Remove(localFilePath)
		return err
	}

	// 12. 进行向量化处理
	err = x.processVectorization(ctx, req.DocumentId, doc.CollectionName, chunks)
	if err != nil {
		g.Log().Errorf(ctx, "向量化处理失败, documentId=%s, err=%v", req.DocumentId, err)
		// 切分成功但向量化失败，设置状态为StatusIndexing
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusIndexing))
		// 删除upload目录下的文件
		os.Remove(localFilePath)
		return err
	}

	g.Log().Infof(ctx, "文档处理完成, documentId=%s", req.DocumentId)

	// 13. 更新文档状态为Active
	err = knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusActive))
	if err != nil {
		g.Log().Errorf(ctx, "更新文档状态失败, documentId=%s, err=%v", req.DocumentId, err)
	}

	// 14. 删除upload目录下的文件
	os.Remove(localFilePath)
	g.Log().Infof(ctx, "删除upload目录文件, path=%s", localFilePath)

	return nil
}

// saveChunksToDatabase 保存chunks到数据库
func (x *Rag) saveChunksToDatabase(ctx context.Context, documentId, collectionName string, chunks []*schema.Document) error {
	if len(chunks) == 0 {
		return nil
	}

	chunkEntities := make([]entity.KnowledgeChunks, len(chunks))
	for i, chunk := range chunks {
		chunkId := uuid.New().String()
		chunkEntities[i] = entity.KnowledgeChunks{
			Id:             chunkId,
			KnowledgeDocId: documentId,
			Content:        chunk.Content,
			CollectionName: collectionName,
			Status:         int(v1.StatusPending),
		}
	}

	err := knowledge.SaveChunksData(ctx, documentId, chunkEntities)
	if err != nil {
		return fmt.Errorf("保存chunks到数据库失败: %w", err)
	}

	return nil
}

// processVectorization 处理向量化
func (x *Rag) processVectorization(ctx context.Context, documentId, collectionName string, chunks []*schema.Document) error {
	// 获取配置
	conf := x.conf

	// 创建Milvus索引器
	idxer, err := indexer.NewIndexer(ctx, conf, collectionName)
	if err != nil {
		return fmt.Errorf("创建Milvus索引器失败: %w", err)
	}

	// 设置上下文，传递必要的信息
	ctx = context.WithValue(ctx, common.DocumentId, documentId)

	// 如果需要知识库ID，从文档信息中获取
	doc, err := knowledge.GetDocumentById(ctx, documentId)
	if err == nil && doc.KnowledgeId != "" {
		ctx = context.WithValue(ctx, common.KnowledgeId, doc.KnowledgeId)
	}

	// 使用索引器进行向量化并存储到Milvus
	chunkIds, err := idxer.Store(ctx, chunks)
	if err != nil {
		return fmt.Errorf("向量化并存储到Milvus失败: %w", err)
	}

	g.Log().Infof(ctx, "向量化处理完成, documentId=%s, collectionName=%s, chunks数量=%d, 成功存储=%d",
		documentId, collectionName, len(chunks), len(chunkIds))

	return nil
}

// fileExists 检查文件是否存在
func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// DeleteChunk 从 Milvus 中删除指定的知识块
func (x *Rag) DeleteChunk(ctx context.Context, collectionName string, chunkID string) error {
	// 构造删除过滤条件
	filter := fmt.Sprintf("id == '%s'", chunkID)

	// 执行删除操作
	deleteOpt := milvusclient.NewDeleteOption(collectionName).WithExpr(filter)
	_, err := x.Client.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("从 Milvus 删除 chunk 失败: %w", err)
	}

	g.Log().Infof(ctx, "成功从 Milvus 删除 chunk, collection=%s, chunkID=%s", collectionName, chunkID)
	return nil
}

// DeleteDocument 从 Milvus 中删除指定的文档
func (x *Rag) DeleteDocument(ctx context.Context, collectionName string, documentID string) error {
	// 构造删除过滤条件
	filter := fmt.Sprintf("document_id == '%s'", documentID)

	// 执行删除操作
	deleteOpt := milvusclient.NewDeleteOption(collectionName).WithExpr(filter)
	_, err := x.Client.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("从 Milvus 删除 document 失败: %w", err)
	}

	g.Log().Infof(ctx, "成功从 Milvus 删除 document, collection=%s, documentID=%s", collectionName, documentID)
	return nil
}
