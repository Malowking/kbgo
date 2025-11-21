package dao

import (
	"context"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// UserDAO 用户数据访问对象
type UserDAO struct{}

var User = &UserDAO{}

// Create 创建用户
func (d *UserDAO) Create(ctx context.Context, user *gormModel.User) error {
	if err := GetDB().WithContext(ctx).Create(user).Error; err != nil {
		g.Log().Errorf(ctx, "创建用户失败: %v", err)
		return err
	}
	return nil
}

// GetByUserID 根据用户ID获取用户
func (d *UserDAO) GetByUserID(ctx context.Context, userID string) (*gormModel.User, error) {
	var user gormModel.User
	if err := GetDB().WithContext(ctx).Where("user_id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		g.Log().Errorf(ctx, "查询用户失败: %v", err)
		return nil, err
	}
	return &user, nil
}

// Update 更新用户
func (d *UserDAO) Update(ctx context.Context, user *gormModel.User) error {
	if err := GetDB().WithContext(ctx).Save(user).Error; err != nil {
		g.Log().Errorf(ctx, "更新用户失败: %v", err)
		return err
	}
	return nil
}
