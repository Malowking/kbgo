package kbgo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/file_store"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/index"
	"github.com/Malowking/kbgo/internal/model/do"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/google/uuid"
)

func (c *ControllerV1) KBCreate(ctx context.Context, req *v1.KBCreateReq) (res *v1.KBCreateRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "KBCreate request received - Name: %s, Description: %s, Category: %s, EmbeddingModelId: %s",
		req.Name, req.Description, req.Category, req.EmbeddingModelId)

	res = &v1.KBCreateRes{}

	// 验证 embedding 模型是否存在且类型为 embedding
	modelConfig := model.Registry.Get(req.EmbeddingModelId)
	if modelConfig == nil {
		return nil, fmt.Errorf("embedding model not found: %s", req.EmbeddingModelId)
	}
	if modelConfig.Type != model.ModelTypeEmbedding {
		return nil, fmt.Errorf("model %s is not an embedding model, type: %s", req.EmbeddingModelId, modelConfig.Type)
	}

	// 获取模型维度
	dimension := 1024 // 默认维度
	if modelConfig.Extra != nil {
		if dim, ok := modelConfig.Extra["dimension"].(float64); ok {
			dimension = int(dim)
		} else if dim, ok := modelConfig.Extra["dimension"].(int); ok {
			dimension = dim
		}
	}
	g.Log().Infof(ctx, "Using embedding model: %s with dimension: %d", modelConfig.Name, dimension)

	// 生成 UUID 作为知识库 ID (使用与项目其他地方相同的格式)
	knowledgeId := "kb_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	// 使用 GORM 模型确保自动填充 CreateTime 和 UpdateTime
	kb := &gormModel.KnowledgeBase{
		ID:               knowledgeId,
		Name:             req.Name,
		Description:      req.Description,
		Category:         req.Category,
		CollectionName:   knowledgeId,          // 使用知识库ID作为默认的CollectionName
		EmbeddingModelId: req.EmbeddingModelId, // 保存绑定的 embedding 模型 ID
		Status:           1,                    // 默认启用
	}

	err = dao.GetDB().WithContext(ctx).Create(kb).Error
	if err != nil {
		return nil, err
	}

	// 创建向量库 collection，传入维度参数
	docIndexSvr := index.GetDocIndexSvr()
	err = docIndexSvr.GetVectorStore().CreateCollection(ctx, knowledgeId, dimension)
	if err != nil {
		// 如果创建向量库 collection 失败，删除已创建的数据库记录并返回错误
		dao.GetDB().WithContext(ctx).Delete(&gormModel.KnowledgeBase{}, "id = ?", knowledgeId)
		return nil, fmt.Errorf("创建向量库 collection 失败: %w", err)
	}
	g.Log().Infof(ctx, "成功创建向量库 collection: %s with dimension: %d", knowledgeId, dimension)

	// 如果使用本地存储，则创建对应的文件夹
	storageType := file_store.GetStorageType()
	if storageType == file_store.StorageTypeLocal {
		// 创建 upload/knowledge_file/{knowledge_id} 目录
		knowledgeDir := filepath.Join("upload", "knowledge_file", knowledgeId)
		if !gfile.Exists(knowledgeDir) {
			err = os.MkdirAll(knowledgeDir, 0755)
			if err != nil {
				g.Log().Errorf(ctx, "创建知识库目录失败: %s, 错误: %v", knowledgeDir, err)
				// 不返回错误，因为数据库记录和向量库 collection 已创建成功
			} else {
				cwd, _ := os.Getwd()
				g.Log().Infof(ctx, "成功创建知识库目录: %s 在 %s", knowledgeDir, cwd)
			}
		}
	}

	res.Id = knowledgeId
	return
}

func (c *ControllerV1) KBDelete(ctx context.Context, req *v1.KBDeleteReq) (res *v1.KBDeleteRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "KBDelete request received - Id: %s", req.Id)

	docIndexSvr := index.GetDocIndexSvr()

	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during KBDelete: %v", r)
		}
	}()

	// 1. 获取该知识库下的所有文档信息（用于删除存储中的文件）
	var documents []gormModel.KnowledgeDocuments
	result := tx.WithContext(ctx).Where("knowledge_id = ?", req.Id).Find(&documents)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 2. 收集需要删除的文件信息（去重）
	type RustFSFile struct {
		Bucket   string
		Location string
		SHA256   string
	}

	type LocalFile struct {
		Path   string
		SHA256 string
	}

	rustfsFiles := make(map[string]RustFSFile) // 使用 map 去重
	localFiles := make(map[string]LocalFile)   // 使用 map 去重

	// 根据存储类型收集需要删除的文件
	storageType := file_store.GetStorageType()
	for _, doc := range documents {
		if doc.SHA256 != "" {
			if storageType == file_store.StorageTypeRustFS {
				// RustFS 存储
				if doc.RustfsBucket != "" && doc.RustfsLocation != "" {
					rustfsFiles[doc.SHA256] = RustFSFile{
						Bucket:   doc.RustfsBucket,
						Location: doc.RustfsLocation,
						SHA256:   doc.SHA256,
					}
				}
			} else {
				// 本地存储
				if doc.LocalFilePath != "" {
					localFiles[doc.SHA256] = LocalFile{
						Path:   doc.LocalFilePath,
						SHA256: doc.SHA256,
					}
				}
			}
		}
	}

	// 3. 删除该知识库下所有文档的 chunks
	for _, doc := range documents {
		result = tx.WithContext(ctx).Where("knowledge_doc_id = ?", doc.ID).Delete(&gormModel.KnowledgeChunks{})
		if result.Error != nil {
			tx.Rollback()
			return nil, result.Error
		}
	}

	// 4. 删除该知识库下的所有文档记录
	result = tx.WithContext(ctx).Where("knowledge_id = ?", req.Id).Delete(&gormModel.KnowledgeDocuments{})
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 5. 删除知识库记录
	result = tx.WithContext(ctx).Where("id = ?", req.Id).Delete(&gormModel.KnowledgeBase{})
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 6. 删除 Milvus collection
	err = docIndexSvr.GetVectorStore().DeleteCollection(ctx, req.Id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	// 7. 事务成功提交后，删除存储中的文件（这个操作失败不影响数据一致性）
	if storageType == file_store.StorageTypeRustFS {
		// 删除 RustFS 文件
		if len(rustfsFiles) > 0 {
			rustfsConfig := file_store.GetRustfsConfig()
			rustfsClient := rustfsConfig.Client

			for _, file := range rustfsFiles {
				err = file_store.DeleteObject(ctx, rustfsClient, file.Bucket, file.Location)
				if err != nil {
					// 记录错误但不返回，因为数据库操作已经成功
					_ = err // 避免未使用变量的警告
				}
			}
		}
	} else {
		// 删除本地文件夹
		if len(localFiles) > 0 {
			// 获取任意一个文件路径来确定知识库文件夹路径
			var anyFilePath string
			for _, file := range localFiles {
				anyFilePath = file.Path
				break
			}

			if anyFilePath != "" {
				// 获取知识库文件夹路径 (knowledge_file/{knowledge_id})
				knowledgeDir := filepath.Dir(anyFilePath)
				g.Log().Infof(ctx, "KBDelete: deleting knowledge directory, path=%s", knowledgeDir)

				// 删除整个知识库文件夹
				err = os.RemoveAll(knowledgeDir)
				if err != nil {
					g.Log().Errorf(ctx, "KBDelete: failed to delete knowledge directory, path=%s, err: %v", knowledgeDir, err)
					// 不返回错误，因为数据库操作已经成功
				} else {
					g.Log().Infof(ctx, "KBDelete: successfully deleted knowledge directory, path=%s", knowledgeDir)
				}
			}
		} else {
			// 即使没有文件，也要删除知识库目录
			knowledgeDir := filepath.Join("upload", "knowledge_file", req.Id)
			if gfile.Exists(knowledgeDir) {
				err = os.RemoveAll(knowledgeDir)
				if err != nil {
					g.Log().Errorf(ctx, "KBDelete: failed to delete knowledge directory, path=%s, err: %v", knowledgeDir, err)
				} else {
					g.Log().Infof(ctx, "KBDelete: successfully deleted knowledge directory, path=%s", knowledgeDir)
				}
			}
		}
	}

	return &v1.KBDeleteRes{}, nil
}

func (c *ControllerV1) KBGetList(ctx context.Context, req *v1.KBGetListReq) (res *v1.KBGetListRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "KBGetList request received - Name: %v, Status: %v, Category: %v",
		req.Name, req.Status, req.Category)

	res = &v1.KBGetListRes{}
	err = dao.KnowledgeBase.Ctx(ctx).Where(do.KnowledgeBase{
		Status:   req.Status,
		Name:     req.Name,
		Category: req.Category,
	}).Scan(&res.List)
	return
}

func (c *ControllerV1) KBGetOne(ctx context.Context, req *v1.KBGetOneReq) (res *v1.KBGetOneRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "KBGetOne request received - Id: %s", req.Id)

	res = &v1.KBGetOneRes{}
	err = dao.KnowledgeBase.Ctx(ctx).WherePri(req.Id).Scan(&res.KnowledgeBase)
	return
}

func (c *ControllerV1) KBUpdate(ctx context.Context, req *v1.KBUpdateReq) (res *v1.KBUpdateRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "KBUpdate request received - Id: %s, Name: %v, Description: %v, Category: %v, Status: %v",
		req.Id, req.Name, req.Description, req.Category, req.Status)

	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during KBUpdate: %v", r)
		}
	}()

	// 更新数据库记录
	updateData := map[string]interface{}{
		"name":        req.Name,
		"status":      req.Status,
		"description": req.Description,
		"category":    req.Category,
	}
	result := tx.WithContext(ctx).Model(&gormModel.KnowledgeBase{}).Where("id = ?", req.Id).Updates(updateData)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	return &v1.KBUpdateRes{}, nil
}

func (c *ControllerV1) KBUpdateStatus(ctx context.Context, req *v1.KBUpdateStatusReq) (res *v1.KBUpdateStatusRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "KBUpdateStatus request received - Id: %s, Status: %d", req.Id, req.Status)

	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during KBUpdateStatus: %v", r)
		}
	}()

	// 检查知识库是否存在
	var count int64
	result := tx.WithContext(ctx).Model(&gormModel.KnowledgeBase{}).Where("id = ?", req.Id).Count(&count)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}
	if count == 0 {
		tx.Rollback()
		return nil, gerror.Newf("knowledge base not found: %s", req.Id)
	}

	// 更新状态
	result = tx.WithContext(ctx).Model(&gormModel.KnowledgeBase{}).Where("id = ?", req.Id).Update("status", req.Status)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	return &v1.KBUpdateStatusRes{}, nil
}
