package rag

import (
	"context"
	"mime/multipart"

	gorag "github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/indexer"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/Malowking/kbgo/internal/model/do"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/cloudwego/eino/components/document"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/Malowking/kbgo/api/rag/v1"
)

func (c *ControllerV1) Indexer(ctx context.Context, req *v1.IndexerReq) (res *v1.IndexerRes, err error) {
	svr := rag.GetRagSvr()
	var fileSHA256 string
	var fileName string
	uri := req.URL

	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during Indexer: %v", r)
		}
	}()

	// 步骤1: 计算 SHA256（无论是 file 还是 URL）
	if req.File != nil {
		// 文件上传
		fileSHA256, err = common.CalculateFileSHA256(req.File.FileHeader)
		if err != nil {
			g.Log().Errorf(ctx, "calculate file SHA256 failed, err=%v", err)
			tx.Rollback()
			return
		}
		fileName = req.File.Filename
	} else if uri != "" {
		// URL 上传
		fileSHA256, err = common.CalculateURLFileSHA256(uri)
		if err != nil {
			g.Log().Errorf(ctx, "calculate URL file SHA256 failed, err=%v", err)
			tx.Rollback()
			return
		}
		// 从 URL 中提取文件名
		fileName = getFileNameFromURL(uri)
		uri = req.URL
	} else {
		err = gerror.New("no file or URL provided")
		tx.Rollback()
		return
	}

	// 步骤2: 检查数据库中是否已经存在相同 SHA256 的文档（使用事务）
	// 先查询是否存在相同 SHA256 和 knowledge_id 的文档
	var count int64
	result := tx.WithContext(ctx).Model(&entity.KnowledgeDocuments{}).Where("sha256 = ? AND knowledge_id = ?", fileSHA256, req.KnowledgeId).Count(&count)
	if result.Error != nil {
		g.Log().Errorf(ctx, "query document by SHA256 failed, err=%v", result.Error)
		tx.Rollback()
		return nil, result.Error
	}

	// 如果是同一个 knowledge_id 且文档已存在，报错
	if count > 0 {
		err = gerror.Newf("文件已存在于当前知识库中，SHA256=%s", fileSHA256)
		g.Log().Errorf(ctx, "document already exists in current knowledge base, SHA256=%s, knowledge_id=%s", fileSHA256, req.KnowledgeId)
		tx.Rollback()
		return nil, err
	}

	// 查询是否存在相同 SHA256 但不同 knowledge_id 的文档
	var existingDocOtherKB entity.KnowledgeDocuments
	result = tx.WithContext(ctx).Where("sha256 = ?", fileSHA256).First(&existingDocOtherKB)
	if result.Error != nil && result.Error.Error() != "record not found" {
		g.Log().Errorf(ctx, "query document by SHA256 (other KB) failed, err=%v", result.Error)
		tx.Rollback()
		return nil, result.Error
	}

	// 如果存在相同 SHA256 但不同 knowledge_id 的文档，复用该文档信息
	if result.Error == nil && existingDocOtherKB.KnowledgeId != req.KnowledgeId {
		g.Log().Infof(ctx, "reusing file with SHA256=%s from another knowledge base", fileSHA256)

		// 获取当前知识库信息
		var kb entity.KnowledgeBase
		result = tx.WithContext(ctx).Where("id = ?", req.KnowledgeId).First(&kb)
		if result.Error != nil {
			g.Log().Errorf(ctx, "get knowledge base failed, err=%v", result.Error)
			tx.Rollback()
			return nil, result.Error
		}

		// 创建新记录，复用 RustFS 信息
		newDoc := entity.KnowledgeDocuments{
			KnowledgeId:    req.KnowledgeId,
			FileName:       existingDocOtherKB.FileName,
			CollectionName: kb.CollectionName,
			SHA256:         fileSHA256,
			RustfsBucket:   existingDocOtherKB.RustfsBucket,
			RustfsLocation: existingDocOtherKB.RustfsLocation,
			IsQA:           req.IsQA,
			Status:         existingDocOtherKB.Status,
		}
		newDoc, err = knowledge.SaveDocumentsInfoWithTx(ctx, tx, newDoc)
		if err != nil {
			g.Log().Errorf(ctx, "SaveDocumentsInfo failed, err=%v", err)
			tx.Rollback()
			return nil, err
		}

		// 提交事务
		if err = tx.Commit().Error; err != nil {
			g.Log().Errorf(ctx, "Indexer: transaction commit failed, err: %v", err)
			return nil, gerror.Newf("failed to commit transaction: %v", err)
		}

		g.Log().Infof(ctx, "created new document reference, document_id=%s, knowledge_id=%s", newDoc.Id, req.KnowledgeId)

		// 返回新文档的 ID
		res = &v1.IndexerRes{
			DocIDs: []string{newDoc.Id},
		}
		return res, nil
	}

	// 步骤3: 文档不存在，需要上传到 RustFS
	rustfsConfig := gorag.GetRustfsConfig()
	var fileHeader *multipart.FileHeader
	if req.File != nil {
		fileHeader = req.File.FileHeader
	}
	info, err := common.UploadToRustFS(ctx, rustfsConfig.Client, rustfsConfig.BucketName, req.KnowledgeId, fileHeader, uri)
	if err != nil {
		g.Log().Errorf(ctx, "upload to RustFS failed, err=%v", err)
		tx.Rollback()
		return
	}
	g.Log().Infof(ctx, "uploaded to RustFS, bucket=%s, location=%s", info.Bucket, info.Key)

	// 步骤4: 获取知识库信息
	var kb entity.KnowledgeBase
	result = tx.WithContext(ctx).Where("id = ?", req.KnowledgeId).First(&kb)
	if result.Error != nil {
		g.Log().Errorf(ctx, "get knowledge base failed, err=%v", result.Error)
		tx.Rollback()
		return nil, result.Error
	}

	// 步骤5: 保存文档信息到数据库（使用事务）
	documents := entity.KnowledgeDocuments{
		KnowledgeId:    req.KnowledgeId,
		FileName:       fileName,
		CollectionName: kb.CollectionName,
		SHA256:         fileSHA256,
		RustfsBucket:   info.Bucket,
		RustfsLocation: info.Key,
		IsQA:           req.IsQA,
		Status:         int(v1.StatusPending),
	}
	documents, err = knowledge.SaveDocumentsInfoWithTx(ctx, tx, documents)
	if err != nil {
		g.Log().Errorf(ctx, "SaveDocumentsInfo failed, err=%v", err)
		tx.Rollback()
		return
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "Indexer: transaction commit failed, err: %v", err)
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	// 步骤6: 设置默认值
	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	overlapSize := req.OverlapSize
	if overlapSize < 0 {
		overlapSize = 100
	}

	// 步骤7: 使用 loader 加载文档
	loader, err := indexer.Loader(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "create loader failed, err=%v", err)
		return
	}

	docs, err := loader.Load(ctx, document.Source{URI: uri})
	if err != nil {
		g.Log().Errorf(ctx, "load document failed, err=%v", err)
		return
	}

	// 步骤8: 执行索引
	indexReq := &gorag.IndexReq{
		Docs:           docs,
		KnowledgeId:    req.KnowledgeId,
		CollectionName: kb.CollectionName,
		DocumentsId:    documents.Id,
		ChunkSize:      chunkSize,
		OverlapSize:    overlapSize,
	}
	ids, err := svr.Index(ctx, indexReq)
	if err != nil {
		return
	}

	res = &v1.IndexerRes{
		DocIDs: ids,
	}
	return
}

// getFileNameFromURL 从 URL 中提取文件名
func getFileNameFromURL(urlStr string) string {
	// 简单实现：取最后一个 / 后的内容
	parts := splitURL(urlStr)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown_file"
}

func splitURL(urlStr string) []string {
	var result []string
	current := ""
	for _, c := range urlStr {
		if c == '/' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
