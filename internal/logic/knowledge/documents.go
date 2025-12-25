package knowledge

import (
	"context"
	"strings"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/internal/dao"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

// SaveDocumentsInfoWithTx 保存文档信息
func SaveDocumentsInfoWithTx(ctx context.Context, tx *gorm.DB, documents gormModel.KnowledgeDocuments) (documentsSave gormModel.KnowledgeDocuments, err error) {
	id := strings.ReplaceAll(uuid.New().String(), "-", "")
	documents.ID = id

	// 转换为 GORM 模型
	gormDoc := gormModel.KnowledgeDocuments{
		ID:             documents.ID,
		KnowledgeId:    documents.KnowledgeId,
		FileName:       documents.FileName,
		FileExtension:  documents.FileExtension, // 添加文件扩展名
		CollectionName: documents.CollectionName,
		SHA256:         documents.SHA256,
		RustfsBucket:   documents.RustfsBucket,
		RustfsLocation: documents.RustfsLocation,
		LocalFilePath:  documents.LocalFilePath, // 添加本地文件路径
		Status:         int8(documents.Status),
	}

	// 如果没有提供事务，则使用默认的数据库连接
	var result *gorm.DB
	if tx != nil {
		result = tx.WithContext(ctx).Create(&gormDoc)
	} else {
		// 使用 DAO 中的 GORM 数据库连接
		result = dao.GetDB().WithContext(ctx).Create(&gormDoc)
	}

	if result.Error != nil {
		g.Log().Errorf(ctx, "保存文档信息失败: %+v, 错误: %v", documents, result.Error)
		return documents, errors.Newf(errors.ErrDatabaseInsert, "保存文档信息失败: %v", result.Error)
	}
	g.Log().Infof(ctx, "文档保存成功, ID: %s", id)
	return documents, nil
}

// UpdateDocumentsStatus 更新文档状态
func UpdateDocumentsStatus(ctx context.Context, documentsId string, status int) error {
	data := map[string]interface{}{
		"status": status,
	}

	err := dao.GetDB().WithContext(ctx).Model(&gormModel.KnowledgeDocuments{}).Where("id = ?", documentsId).Updates(data).Error
	if err != nil {
		g.Log().Errorf(ctx, "更新文档状态失败: ID=%s, 错误: %v", documentsId, err)
	}

	return err
}

//// UpdateDocumentsLocalPath 更新文档的本地文件路径
//func UpdateDocumentsLocalPath(ctx context.Context, documentsId string, localPath string) error {
//	data := map[string]interface{}{
//		"local_file_path": localPath,
//	}
//
//	err := dao.GetDB().WithContext(ctx).Model(&gormModel.KnowledgeDocuments{}).Where("id = ?", documentsId).Updates(data).Error
//	if err != nil {
//		g.Log().Errorf(ctx, "更新文档本地文件路径失败: ID=%s, 路径=%s, 错误: %v", documentsId, localPath, err)
//	}
//
//	return err
//}

// GetDocumentById 根据ID获取文档信息
func GetDocumentById(ctx context.Context, id string) (document gormModel.KnowledgeDocuments, err error) {
	err = dao.GetDB().WithContext(ctx).Where("id = ?", id).First(&document).Error
	if err != nil {
		g.Log().Errorf(ctx, "获取文档信息失败: ID=%s, 错误: %v", id, err)
		return document, errors.Newf(errors.ErrDatabaseQuery, "获取文档信息失败: %v", err)
	}

	return document, nil
}

// GetDocumentBySHA256 根据知识库ID和SHA256获取文档信息
func GetDocumentBySHA256(ctx context.Context, knowledgeId, sha256 string) (document gormModel.KnowledgeDocuments, err error) {
	err = dao.GetDB().WithContext(ctx).Where("knowledge_id = ? AND sha256 = ?", knowledgeId, sha256).First(&document).Error
	if err != nil {
		// 当没有找到匹配的记录时，返回空文档，无错误
		if err == gorm.ErrRecordNotFound {
			return document, nil // 返回空文档，无错误
		}

		g.Log().Errorf(ctx, "根据SHA256获取文档信息失败: KnowledgeId=%s, SHA256=%s, 错误: %v", knowledgeId, sha256, err)
		return document, errors.Newf(errors.ErrDatabaseQuery, "根据SHA256获取文档信息失败: %v", err)
	}

	return document, nil
}

// GetDocumentsList 获取文档列表
func GetDocumentsList(ctx context.Context, where gormModel.KnowledgeDocuments, page int, pageSize int) (documents []gormModel.KnowledgeDocuments, total int, err error) {
	// 参数验证和默认值设置
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	query := dao.GetDB().WithContext(ctx).Model(&gormModel.KnowledgeDocuments{})
	if where.KnowledgeId != "" {
		query = query.Where("knowledge_id = ?", where.KnowledgeId)
	}

	var count int64
	err = query.Count(&count).Error
	total = int(count)
	if err != nil {
		g.Log().Errorf(ctx, "获取文档总数失败: %v", err)
		return nil, 0, errors.Newf(errors.ErrDatabaseQuery, "获取文档总数失败: %v", err)
	}

	if total == 0 {
		return nil, 0, nil
	}

	offset := (page - 1) * pageSize
	err = query.Offset(offset).Limit(pageSize).Order("create_time desc").Find(&documents).Error
	if err != nil {
		g.Log().Errorf(ctx, "获取文档列表失败: %v", err)
		return nil, 0, errors.Newf(errors.ErrDatabaseQuery, "获取文档列表失败: %v", err)
	}

	return documents, total, nil
}

// DeleteDocumentWithTx 删除文档及其相关数据
func DeleteDocumentWithTx(ctx context.Context, tx *gorm.DB, id string) error {
	// 先删除文档块
	result := tx.WithContext(ctx).Where("knowledge_doc_id = ?", id).Delete(&gormModel.KnowledgeChunks{})
	if result.Error != nil {
		g.Log().Errorf(ctx, "删除文档块失败: ID=%s, 错误: %v", id, result.Error)
		return errors.Newf(errors.ErrDatabaseDelete, "删除文档块失败: %v", result.Error)
	}

	// 再删除文档
	result = tx.WithContext(ctx).Where("id = ?", id).Delete(&gormModel.KnowledgeDocuments{})
	if result.Error != nil {
		g.Log().Errorf(ctx, "删除文档失败: ID=%s, 错误: %v", id, result.Error)
		return errors.Newf(errors.ErrDatabaseDelete, "删除文档失败: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New(errors.ErrDocumentNotFound, "文档不存在")
	}

	g.Log().Infof(ctx, "文档删除成功: ID=%s", id)
	return nil
}
