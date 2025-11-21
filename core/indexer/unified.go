package indexer

import (
	"context"
	"fmt"
	"os"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/google/uuid"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
)

// UnifiedIndexReq 统一索引请求参数
type UnifiedIndexReq struct {
	DocumentId  string // 文档ID
	ChunkSize   int    // 文档分块大小
	OverlapSize int    // 分块重叠大小
	Separator   string // 自定义分隔符
}

// UnifiedDocumentIndex 统一处理文档索引（包含状态检查和存储类型处理）
func UnifiedDocumentIndex(ctx context.Context, req *UnifiedIndexReq) error {
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

	// 3. 获取存储类型配置
	storageType := common.GetStorageType()
	uploadDir := common.GetUploadDirByFileType(doc.FileExtension)

	localFilePath := doc.LocalFilePath

	// 4. 如果是RustFS存储类型，需要先下载文件到本地
	if storageType == common.StorageTypeRustFS {
		// 确保本地有文件路径，如果没有则创建临时路径
		if localFilePath == "" {
			localFilePath = gfile.Join(uploadDir, fmt.Sprintf("%s%s", uuid.New().String(), doc.FileExtension))
		}

		// 从RustFS下载文件到本地临时路径
		rustfsConfig := common.GetRustfsConfig()
		err = common.DownloadFileToPath(ctx, rustfsConfig.Client, doc.RustfsBucket, doc.RustfsLocation, localFilePath)
		if err != nil {
			g.Log().Errorf(ctx, "从RustFS下载文件失败, documentId=%s, bucket=%s, location=%s, err=%v",
				req.DocumentId, doc.RustfsBucket, doc.RustfsLocation, err)
			knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
			return err
		}
		g.Log().Infof(ctx, "从RustFS下载文件成功, documentId=%s, localPath=%s", req.DocumentId, localFilePath)
	}

	// 5. 检查本地文件是否存在
	if localFilePath == "" || !fileExists(localFilePath) {
		err = fmt.Errorf("本地文件不存在, path=%s", localFilePath)
		g.Log().Errorf(ctx, "本地文件不存在, documentId=%s, path=%s", req.DocumentId, localFilePath)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		return err
	}

	// 6. 创建Loader
	loader, err := Loader(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "创建Loader失败, documentId=%s, err=%v", req.DocumentId, err)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		// 清理临时文件（如果是从RustFS下载的）
		if storageType == common.StorageTypeRustFS {
			os.Remove(localFilePath)
		}
		return err
	}

	// 7. 加载文档
	docs, err := loader.Load(ctx, document.Source{URI: localFilePath})
	if err != nil {
		g.Log().Errorf(ctx, "加载文档失败, documentId=%s, err=%v", req.DocumentId, err)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		// 清理临时文件（如果是从RustFS下载的）
		if storageType == common.StorageTypeRustFS {
			os.Remove(localFilePath)
		}
		return err
	}

	// 8. 创建Transformer进行切分
	var transformer document.Transformer
	if req.Separator != "" {
		// 使用自定义分隔符
		transformer, err = SeparatorSplitter(ctx, req.Separator, req.ChunkSize, req.OverlapSize)
	} else {
		// 使用默认分隔符
		transformer, err = NewTransformer(ctx, req.ChunkSize, req.OverlapSize)
	}
	if err != nil {
		g.Log().Errorf(ctx, "创建Transformer失败, documentId=%s, err=%v", req.DocumentId, err)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		// 清理临时文件（如果是从RustFS下载的）
		if storageType == common.StorageTypeRustFS {
			os.Remove(localFilePath)
		}
		return err
	}

	// 9. 切分文档
	chunks, err := transformer.Transform(ctx, docs)
	if err != nil {
		g.Log().Errorf(ctx, "文档切分失败, documentId=%s, err=%v", req.DocumentId, err)
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusFailed))
		// 清理临时文件（如果是从RustFS下载的）
		if storageType == common.StorageTypeRustFS {
			os.Remove(localFilePath)
		}
		return err
	}

	g.Log().Infof(ctx, "文档切分完成, documentId=%s, chunk数量=%d", req.DocumentId, len(chunks))

	// 10. 保存chunks到数据库
	err = saveChunksToDatabase(ctx, req.DocumentId, doc.CollectionName, chunks)
	if err != nil {
		g.Log().Errorf(ctx, "保存chunks到数据库失败, documentId=%s, err=%v", req.DocumentId, err)
		// 即使保存失败，状态也要设置为StatusIndexing（切分成功但向量化失败）
		knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusIndexing))
		// 清理临时文件（如果是从RustFS下载的）
		if storageType == common.StorageTypeRustFS {
			os.Remove(localFilePath)
		}
		return err
	}

	g.Log().Infof(ctx, "文档处理完成, documentId=%s", req.DocumentId)

	// 11. 更新文档状态为Active
	err = knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusActive))
	if err != nil {
		g.Log().Errorf(ctx, "更新文档状态失败, documentId=%s, err=%v", req.DocumentId, err)
	}

	// 12. 如果是从RustFS下载的临时文件，删除它
	if storageType == common.StorageTypeRustFS {
		os.Remove(localFilePath)
		g.Log().Infof(ctx, "删除临时文件, path=%s", localFilePath)
	}

	return nil
}

// saveChunksToDatabase 保存chunks到数据库
func saveChunksToDatabase(ctx context.Context, documentId, collectionName string, chunks []*schema.Document) error {
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

// fileExists 检查文件是否存在
func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}
