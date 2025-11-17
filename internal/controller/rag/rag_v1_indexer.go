package rag

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	gorag "github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/indexer"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/Malowking/kbgo/internal/model/entity"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/minio/minio-go/v7"

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
	} else {
		err = gerror.New("no file or URL provided")
		tx.Rollback()
		return
	}

	// 步骤2: 检查数据库中是否已经存在相同 SHA256 的文档（使用事务）
	// 先查询是否存在相同 SHA256 和 knowledge_id 的文档
	var count int64
	result := tx.WithContext(ctx).Model(&gormModel.KnowledgeDocuments{}).Where("sha256 = ? AND knowledge_id = ?", fileSHA256, req.KnowledgeId).Count(&count)
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

	// 步骤3: 获取知识库信息
	var kb gormModel.KnowledgeBase
	result = tx.WithContext(ctx).Model(&gormModel.KnowledgeBase{}).Where("id = ?", req.KnowledgeId).First(&kb)
	if result.Error != nil {
		g.Log().Errorf(ctx, "get knowledge base failed, err=%v", result.Error)
		tx.Rollback()
		return nil, result.Error
	}

	// 步骤4: 根据配置决定存储方式
	storageType := gorag.GetStorageType()
	var fileHeader *multipart.FileHeader
	var localFilePath string
	var documents entity.KnowledgeDocuments

	if storageType == gorag.StorageTypeRustFS {
		// 使用 RustFS 存储
		rustfsConfig := gorag.GetRustfsConfig()
		if req.File != nil {
			fileHeader = req.File.FileHeader
		}
		var info minio.UploadInfo
		info, err = common.UploadToRustFS(ctx, rustfsConfig.Client, rustfsConfig.BucketName, req.KnowledgeId, fileHeader, uri)
		if err != nil {
			g.Log().Errorf(ctx, "upload to RustFS failed, err=%v", err)
			tx.Rollback()
			return
		}
		g.Log().Infof(ctx, "uploaded to RustFS, bucket=%s, location=%s", info.Bucket, info.Key)

		// 保存到数据库
		documents = entity.KnowledgeDocuments{
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
	} else {
		// 使用本地存储
		if req.File != nil {
			fileHeader = req.File.FileHeader
		}

		// 创建目录 knowledge_file/知识库id/
		localDir := filepath.Join("knowledge_file", req.KnowledgeId)
		if !gfile.Exists(localDir) {
			err = os.MkdirAll(localDir, 0755)
			if err != nil {
				g.Log().Errorf(ctx, "create local directory failed, dir=%s, err=%v", localDir, err)
				tx.Rollback()
				return nil, err
			}
		}

		// 保存文件路径
		localFilePath = filepath.Join(localDir, fileName)

		// 保存文件到本地
		if req.File != nil {
			// 保存上传的文件
			err = saveUploadedFile(req.File.FileHeader, localFilePath)
			if err != nil {
				g.Log().Errorf(ctx, "save uploaded file failed, path=%s, err=%v", localFilePath, err)
				tx.Rollback()
				return nil, err
			}
		} else if uri != "" {
			// 下载URL文件并保存
			err = downloadAndSaveFile(uri, localFilePath)
			if err != nil {
				g.Log().Errorf(ctx, "download and save file failed, url=%s, path=%s, err=%v", uri, localFilePath, err)
				tx.Rollback()
				return nil, err
			}
		}

		// 保存到数据库
		documents = entity.KnowledgeDocuments{
			KnowledgeId:    req.KnowledgeId,
			FileName:       fileName,
			CollectionName: kb.CollectionName,
			SHA256:         fileSHA256,
			RustfsBucket:   "", // 本地存储时这些字段为空
			RustfsLocation: "",
			IsQA:           req.IsQA,
			Status:         int(v1.StatusPending),
			LocalFilePath:  localFilePath, // 保存本地文件路径
		}
		documents, err = knowledge.SaveDocumentsInfoWithTx(ctx, tx, documents)
		if err != nil {
			g.Log().Errorf(ctx, "SaveDocumentsInfo failed, err=%v", err)
			tx.Rollback()
			return
		}
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "Indexer: transaction commit failed, err: %v", err)
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	// 步骤5: 设置默认值
	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	overlapSize := req.OverlapSize
	if overlapSize < 0 {
		overlapSize = 100
	}

	// 步骤6: 使用 loader 加载文档
	var loader document.Loader
	var rustfsConfig *gorag.RustfsConfig
	if storageType == gorag.StorageTypeRustFS {
		rustfsConfig = gorag.GetRustfsConfig()
		loader, err = indexer.Loader(ctx, rustfsConfig.Client, rustfsConfig.BucketName)
	} else {
		// 本地存储使用文件加载器
		loader, err = createLocalLoader(ctx)
	}

	if err != nil {
		g.Log().Errorf(ctx, "create loader failed, err=%v", err)
		return
	}

	// 根据存储类型和文件来源使用不同的加载方式
	var docs []*schema.Document
	if storageType == gorag.StorageTypeRustFS {
		if req.File != nil {
			// 对于本地上传的文件，构造 rustfs URI 直接加载
			rustfsURI := fmt.Sprintf("rustfs://%s/%s/%s", rustfsConfig.BucketName, req.KnowledgeId, req.File.Filename)
			docs, err = loader.Load(ctx, document.Source{URI: rustfsURI})
		} else {
			// 对于URL文件，使用原来的URL方式加载
			docs, err = loader.Load(ctx, document.Source{URI: uri})
		}
	} else {
		// 本地存储直接加载本地文件
		docs, err = loader.Load(ctx, document.Source{URI: localFilePath})
	}

	if err != nil {
		g.Log().Errorf(ctx, "load document failed, err=%v", err)
		return
	}

	// 步骤7: 执行索引
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

// saveUploadedFile 保存上传的文件到本地
func saveUploadedFile(fileHeader *multipart.FileHeader, filePath string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 使用 io.Copy 替代 gfile.Copy
	_, err = io.Copy(dst, src)
	return err
}

// downloadAndSaveFile 下载URL文件并保存到本地
func downloadAndSaveFile(url, filePath string) error {
	resp, err := g.Client().Get(context.Background(), url)
	if err != nil {
		return err
	}
	defer resp.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 使用 io.Copy 替代 gfile.Copy
	_, err = io.Copy(file, resp.Body)
	return err
}

// createLocalLoader 创建本地文件加载器
func createLocalLoader(ctx context.Context) (document.Loader, error) {
	loader, err := indexer.Loader(ctx, nil, "")
	return loader, err
}
