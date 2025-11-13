package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/model/entity"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

// SaveDocumentsInfoWithTx 保存文档信息（事务版本）
func SaveDocumentsInfoWithTx(ctx context.Context, tx *gorm.DB, documents entity.KnowledgeDocuments) (documentsSave entity.KnowledgeDocuments, err error) {
	id := strings.ReplaceAll(uuid.New().String(), "-", "")
	documents.Id = id

	// 转换为 GORM 模型（GORM 会自动设置 CreateTime 和 UpdateTime）
	gormDoc := gormModel.KnowledgeDocuments{
		ID:             documents.Id,
		KnowledgeId:    documents.KnowledgeId,
		FileName:       documents.FileName,
		CollectionName: documents.CollectionName,
		SHA256:         documents.SHA256,
		RustfsBucket:   documents.RustfsBucket,
		RustfsLocation: documents.RustfsLocation,
		IsQA:           int8(documents.IsQA),
		Status:         int8(documents.Status),
	}

	result := tx.WithContext(ctx).Create(&gormDoc)
	if result.Error != nil {
		g.Log().Errorf(ctx, "保存文档信息失败: %+v, 错误: %v", documents, result.Error)
		return documents, fmt.Errorf("保存文档信息失败: %w", result.Error)
	}
	g.Log().Infof(ctx, "文档保存成功, ID: %s", id)
	return documents, nil
}

// UpdateDocumentsStatus 更新文档状态
func UpdateDocumentsStatus(ctx context.Context, documentsId string, status int) error {
	data := g.Map{
		"status": status,
	}

	_, err := dao.KnowledgeDocuments.Ctx(ctx).Where("id", documentsId).Data(data).Update()
	if err != nil {
		g.Log().Errorf(ctx, "更新文档状态失败: ID=%d, 错误: %v", documentsId, err)
	}

	return err
}

// GetDocumentById 根据ID获取文档信息
func GetDocumentById(ctx context.Context, id string) (document entity.KnowledgeDocuments, err error) {
	g.Log().Debugf(ctx, "获取文档信息: ID=%d", id)

	err = dao.KnowledgeDocuments.Ctx(ctx).Where("id", id).Scan(&document)
	if err != nil {
		g.Log().Errorf(ctx, "获取文档信息失败: ID=%d, 错误: %v", id, err)
		return document, fmt.Errorf("获取文档信息失败: %w", err)
	}

	return document, nil
}

// GetDocumentsList 获取文档列表
func GetDocumentsList(ctx context.Context, where entity.KnowledgeDocuments, page int, pageSize int) (documents []entity.KnowledgeDocuments, total int, err error) {
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

	model := dao.KnowledgeDocuments.Ctx(ctx)
	if where.KnowledgeId != "" {
		model = model.Where("knowledge_Id", where.KnowledgeId)
	}

	total, err = model.Count()
	if err != nil {
		g.Log().Errorf(ctx, "获取文档总数失败: %v", err)
		return nil, 0, fmt.Errorf("获取文档总数失败: %w", err)
	}

	if total == 0 {
		return nil, 0, nil
	}

	err = model.Page(page, pageSize).
		Order("create_time desc").
		Scan(&documents)
	if err != nil {
		g.Log().Errorf(ctx, "获取文档列表失败: %v", err)
		return nil, 0, fmt.Errorf("获取文档列表失败: %w", err)
	}

	return documents, total, nil
}

// DeleteDocumentWithTx 删除文档及其相关数据（事务版本）
func DeleteDocumentWithTx(ctx context.Context, tx *gorm.DB, id string) error {
	g.Log().Debugf(ctx, "删除文档: ID=%s", id)

	// 先删除文档块
	result := tx.WithContext(ctx).Where("knowledge_doc_id = ?", id).Delete(&gormModel.KnowledgeChunks{})
	if result.Error != nil {
		g.Log().Errorf(ctx, "删除文档块失败: ID=%s, 错误: %v", id, result.Error)
		return fmt.Errorf("删除文档块失败: %w", result.Error)
	}

	// 再删除文档
	result = tx.WithContext(ctx).Where("id = ?", id).Delete(&gormModel.KnowledgeDocuments{})
	if result.Error != nil {
		g.Log().Errorf(ctx, "删除文档失败: ID=%s, 错误: %v", id, result.Error)
		return fmt.Errorf("删除文档失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("文档不存在")
	}

	g.Log().Infof(ctx, "文档删除成功: ID=%s", id)
	return nil
}
