package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/exp/slices"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/gosimple/slug"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB) *repository {
	return &repository{
		db: db,
	}
}

type repository struct {
	db *gorm.DB
}

func (r repository) Create(ctx context.Context, d *model.Database) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	return r.db.WithContext(ctx).Create(&d).Error
}

func (r repository) Save(ctx context.Context, d *model.Database) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	s := fmt.Sprintf("%s/%s", d.GroupName, d.Name)
	d.Slug = slug.Make(s)

	err := r.db.WithContext(ctx).Save(&d).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("database named %q already exists", d.Name)
	}

	return err
}

func (r repository) FindById(ctx context.Context, id uint) (*model.Database, error) {
	var d *model.Database
	err := r.db.
		WithContext(ctx).
		Preload("Lock").
		First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("database not found by id: %d", id)
	}
	return d, err
}

func (r repository) UpdateId(ctx context.Context, oldID, newID uint) error {
	return r.db.
		WithContext(ctx).
		Model(&model.Database{}).
		Where("id = ?", oldID).
		Update("id", newID).
		Error
}

func (r repository) FindBySlug(ctx context.Context, slug string) (*model.Database, error) {
	var d *model.Database
	err := r.db.
		WithContext(ctx).
		Preload("Lock").
		Where("slug = ?", slug).
		First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("database not found by slug: %q", slug)
	}
	return d, err
}

func (r repository) Lock(ctx context.Context, databaseId, instanceId, userId uint) (*model.Lock, error) {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	var lock *model.Lock
	errTx := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var d *model.Database
		err := tx.
			Preload("Lock").
			First(&d, databaseId).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				err = errdef.NewNotFound("database not found by id: %d", databaseId)
			}
			return err
		}

		if d.Lock != nil && d.Lock.InstanceID != 0 {
			return errdef.NewBadRequest("database already locked by user %q and instance %q", userId, d.Lock.InstanceID)
		}

		lock = &model.Lock{
			DatabaseID: databaseId,
			InstanceID: instanceId,
			UserID:     userId,
		}
		return tx.Create(lock).Error
	})

	return lock, errTx
}

func (r repository) Unlock(ctx context.Context, databaseId uint) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	db := r.db.WithContext(ctx).Unscoped().Delete(&model.Lock{}, "database_id = ?", databaseId)
	if db.Error != nil {
		return db.Error
	}

	if db.RowsAffected < 1 {
		return errdef.NewNotFound("lock not found by database id: %d", databaseId)
	}

	return nil
}

func (r repository) Delete(ctx context.Context, id uint) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	database, err := r.FindById(ctx, id)
	if err != nil {
		return err
	}

	if database.Lock != nil {
		return errdef.NewBadRequest("database is locked")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete external downloads
		var downloads []model.ExternalDownload
		err := tx.
			Where("database_id = ?", id).
			Find(&downloads).Error
		if err != nil {
			return err
		}

		for _, download := range downloads {
			err := tx.Unscoped().Delete(&model.ExternalDownload{}, download.UUID).Error
			if err != nil {
				return err
			}
		}

		// Delete database
		err = tx.Unscoped().Delete(&model.Database{}, id).Error
		if err != nil {
			return err
		}
		return nil
	})
}

func (r repository) FindByGroupNames(ctx context.Context, groupNames []string) ([]model.Database, error) {
	var databases []model.Database

	query := r.db
	isAdmin := slices.Contains(groupNames, model.AdministratorGroupName)
	if !isAdmin {
		query = query.Where("group_name IN ?", groupNames)
	}

	err := query.
		Order("updated_at desc").
		Find(&databases).Error

	return databases, err
}

func (r repository) Update(ctx context.Context, d *model.Database) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	return r.db.WithContext(ctx).Save(d).Error
}

func (r repository) CreateExternalDownload(ctx context.Context, databaseID uint, expirationInSeconds uint) (*model.ExternalDownload, error) {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	externalDownload := &model.ExternalDownload{
		UUID:       uuid.New(),
		Expiration: uint(time.Now().Unix()) + expirationInSeconds,
		DatabaseID: databaseID,
	}

	err := r.db.WithContext(ctx).Save(externalDownload).Error

	return externalDownload, err
}

func (r repository) FindExternalDownload(ctx context.Context, uuid uuid.UUID) (*model.ExternalDownload, error) {
	var d *model.ExternalDownload
	err := r.db.
		WithContext(ctx).
		Where("expiration > ?", time.Now().Unix()).
		First(&d, uuid).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("external download not found by id: %q", uuid)
	}
	return d, err
}

func (r repository) PurgeExternalDownload(ctx context.Context) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	var d *model.ExternalDownload
	err := r.db.
		WithContext(ctx).
		Where("expiration < ?", time.Now().Unix()).
		Delete(&d).Error
	return err
}
