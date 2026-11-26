package notification

import (
	"context"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB) *repository {
	return &repository{db: db}
}

type repository struct {
	db *gorm.DB
}

func (r repository) create(ctx context.Context, n *model.Notification) error {
	ctx = context.WithoutCancel(ctx)
	return r.db.WithContext(ctx).Create(n).Error
}

func (r repository) findByUserID(ctx context.Context, userID uint) ([]model.Notification, error) {
	var notifications []model.Notification
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&notifications).Error
	return notifications, err
}

// findDeployments returns the deployments with the given ids, leaving out presets, as their id and group.
func (r repository) findDeployments(ctx context.Context, ids []uint) ([]model.Deployment, error) {
	var deployments []model.Deployment
	err := r.db.WithContext(ctx).
		Select("id", "group_name").
		Where("id IN ? AND preset = false", ids).
		Find(&deployments).Error
	return deployments, err
}

// findDatabases returns the databases with the given ids as their id and group.
func (r repository) findDatabases(ctx context.Context, ids []uint) ([]model.Database, error) {
	var databases []model.Database
	err := r.db.WithContext(ctx).
		Select("id", "group_name").
		Where("id IN ?", ids).
		Find(&databases).Error
	return databases, err
}

func (r repository) markRead(ctx context.Context, id, userID uint) error {
	ctx = context.WithoutCancel(ctx)
	return r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("read", true).Error
}

func (r repository) markAllRead(ctx context.Context, userID uint) error {
	ctx = context.WithoutCancel(ctx)
	return r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ? AND read = false", userID).
		Update("read", true).Error
}
