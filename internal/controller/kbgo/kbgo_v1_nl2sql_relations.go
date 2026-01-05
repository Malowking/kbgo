package kbgo

import (
	"context"
	"fmt"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/dao"
	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// NL2SQLCreateRelation 创建表关系
func (c *ControllerV1) NL2SQLCreateRelation(ctx context.Context, req *v1.NL2SQLCreateRelationReq) (res *v1.NL2SQLCreateRelationRes, err error) {
	db := dao.GetDB()

	// 1. 验证数据源是否存在
	var ds dbgorm.NL2SQLDataSource
	if err := db.First(&ds, "id = ?", req.DatasourceID).Error; err != nil {
		return nil, fmt.Errorf("数据源不存在")
	}

	// 2. 验证源表和目标表是否存在
	var fromTable, toTable dbgorm.NL2SQLTable
	if err := db.First(&fromTable, "id = ? AND datasource_id = ?", req.FromTableID, req.DatasourceID).Error; err != nil {
		return nil, fmt.Errorf("源表不存在")
	}
	if err := db.First(&toTable, "id = ? AND datasource_id = ?", req.ToTableID, req.DatasourceID).Error; err != nil {
		return nil, fmt.Errorf("目标表不存在")
	}

	// 3. 验证列是否存在
	var fromColumn, toColumn dbgorm.NL2SQLColumn
	if err := db.Where("table_id = ? AND column_name = ?", req.FromTableID, req.FromColumn).First(&fromColumn).Error; err != nil {
		return nil, fmt.Errorf("源列不存在: %s", req.FromColumn)
	}
	if err := db.Where("table_id = ? AND column_name = ?", req.ToTableID, req.ToColumn).First(&toColumn).Error; err != nil {
		return nil, fmt.Errorf("目标列不存在: %s", req.ToColumn)
	}

	// 4. 生成 RelationID
	relationID := fmt.Sprintf("rel_%s_%s", fromTable.Name, req.FromColumn)

	// 5. 检查是否已存在相同的关系
	var existingRelation dbgorm.NL2SQLRelation
	if err := db.Where("datasource_id = ? AND relation_id = ?", req.DatasourceID, relationID).First(&existingRelation).Error; err == nil {
		return nil, fmt.Errorf("关系已存在: %s", relationID)
	}

	// 6. 创建关系记录
	relation := &dbgorm.NL2SQLRelation{
		DatasourceID: req.DatasourceID,
		RelationID:   relationID,
		FromTableID:  req.FromTableID,
		FromColumn:   req.FromColumn,
		ToTableID:    req.ToTableID,
		ToColumn:     req.ToColumn,
		RelationType: req.RelationType,
		JoinType:     req.JoinType,
		Description:  req.Description,
	}

	if relation.Description == "" {
		relation.Description = fmt.Sprintf("%s.%s -> %s.%s", fromTable.Name, req.FromColumn, toTable.Name, req.ToColumn)
	}

	if err := db.Create(relation).Error; err != nil {
		return nil, fmt.Errorf("创建关系失败: %w", err)
	}

	g.Log().Infof(ctx, "Relation created: %s", relation.Description)

	return &v1.NL2SQLCreateRelationRes{
		RelationID: relation.ID,
		Message:    "关系创建成功",
	}, nil
}

// NL2SQLListRelations 查询表关系列表
func (c *ControllerV1) NL2SQLListRelations(ctx context.Context, req *v1.NL2SQLListRelationsReq) (res *v1.NL2SQLListRelationsRes, err error) {
	db := dao.GetDB()

	var relations []dbgorm.NL2SQLRelation
	if err := db.Where("datasource_id = ?", req.DatasourceID).Find(&relations).Error; err != nil {
		return nil, fmt.Errorf("查询关系失败: %w", err)
	}

	// 转换为返回格式，需要查询表名
	relationInfos := make([]v1.NL2SQLRelationInfo, 0, len(relations))
	for _, rel := range relations {
		var fromTable, toTable dbgorm.NL2SQLTable
		db.First(&fromTable, "id = ?", rel.FromTableID)
		db.First(&toTable, "id = ?", rel.ToTableID)

		relationInfos = append(relationInfos, v1.NL2SQLRelationInfo{
			ID:            rel.ID,
			RelationID:    rel.RelationID,
			FromTableName: fromTable.Name,
			FromColumn:    rel.FromColumn,
			ToTableName:   toTable.Name,
			ToColumn:      rel.ToColumn,
			RelationType:  rel.RelationType,
			JoinType:      rel.JoinType,
			Description:   rel.Description,
			CreateTime:    rel.CreateTime.Format("2006-01-02 15:04:05"),
		})
	}

	return &v1.NL2SQLListRelationsRes{
		Relations: relationInfos,
	}, nil
}

// NL2SQLDeleteRelation 删除表关系
func (c *ControllerV1) NL2SQLDeleteRelation(ctx context.Context, req *v1.NL2SQLDeleteRelationReq) (res *v1.NL2SQLDeleteRelationRes, err error) {
	db := dao.GetDB()

	// 查找关系
	var relation dbgorm.NL2SQLRelation
	if err := db.First(&relation, "id = ?", req.RelationID).Error; err != nil {
		return &v1.NL2SQLDeleteRelationRes{
			Success: false,
			Message: "关系不存在",
		}, nil
	}

	// 删除关系
	if err := db.Delete(&relation).Error; err != nil {
		return nil, fmt.Errorf("删除关系失败: %w", err)
	}

	g.Log().Infof(ctx, "Relation deleted: %s", relation.Description)

	return &v1.NL2SQLDeleteRelationRes{
		Success: true,
		Message: "关系删除成功",
	}, nil
}
