package cluster

import (
	"context"
	"errors"
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB) *repository {
	return &repository{db}
}

type repository struct {
	db *gorm.DB
}

func (r repository) find(ctx context.Context, id uint) (model.Cluster, error) {
	var cluster model.Cluster
	err := r.db.
		WithContext(ctx).
		Preload("Groups").
		Where("id = ?", id).
		First(&cluster).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Cluster{}, errdef.NewNotFound("cluster with id %d doesn't exist", id)
	}

	if err != nil {
		return model.Cluster{}, fmt.Errorf("failed to find cluster: %v", err)
	}

	return cluster, nil
}

func (r repository) findAll(ctx context.Context) ([]model.Cluster, error) {
	var clusters []model.Cluster
	err := r.db.
		WithContext(ctx).
		Preload("Groups").
		Order("updated_at desc").
		Find(&clusters).Error
	return clusters, err
}

func (r repository) save(ctx context.Context, cluster *model.Cluster) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	err := r.db.WithContext(ctx).Save(cluster).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("cluster name already exists: %s", err)
	}

	return err
}

func (r repository) update(ctx context.Context, cluster model.Cluster) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	err := r.db.WithContext(ctx).Save(&cluster).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("cluster name already exists: %s", err)
	}

	return err
}

func (r repository) delete(ctx context.Context, cluster model.Cluster) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	return r.db.WithContext(ctx).Delete(&cluster).Error
}

func (r repository) addGroup(ctx context.Context, cluster model.Cluster, group model.Group) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	return r.db.WithContext(ctx).Model(&cluster).Association("Groups").Append([]model.Group{group})
}

func (r repository) removeGroup(ctx context.Context, cluster model.Cluster, group model.Group) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	return r.db.WithContext(ctx).Model(&cluster).Association("Groups").Delete([]model.Group{group})
}

func (r repository) findOrCreate(ctx context.Context, cluster model.Cluster) (model.Cluster, error) {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	var c model.Cluster
	err := r.db.
		WithContext(ctx).
		Where(model.Cluster{Name: cluster.Name}).
		Attrs(model.Cluster{Description: cluster.Description, Configuration: cluster.Configuration}).
		FirstOrCreate(&c).Error
	return c, err
}
