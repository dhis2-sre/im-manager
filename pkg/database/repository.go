package database

import (
	"errors"
	"fmt"
	"time"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/gosimple/slug"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

func NewRepository(db *gorm.DB) *repository {
	return &repository{db}
}

type repository struct {
	db *gorm.DB
}

func (r repository) Create(d *model.Database) error {
	return r.db.Create(&d).Error
}

func (r repository) Save(d *model.Database) error {
	s := fmt.Sprintf("%s/%s", d.GroupName, d.Name)
	d.Slug = slug.Make(s)
	return r.db.Save(&d).Error
}

func (r repository) FindById(id uint) (*model.Database, error) {
	var d *model.Database
	err := r.db.
		Preload("Lock").
		First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("database not found by id: %d", id)
	}
	return d, err
}

func (r repository) FindBySlug(slug string) (*model.Database, error) {
	var d *model.Database
	err := r.db.
		Preload("Lock").
		Where("slug = ?", slug).
		First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("database not found by slug: %q", slug)
	}
	return d, err
}

func (r repository) Lock(id, instanceId, userId uint) (*model.Lock, error) {
	var lock *model.Lock

	errTx := r.db.Transaction(func(tx *gorm.DB) error {
		var d *model.Database
		err := tx.
			Preload("Lock").
			First(&d, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				err = errdef.NewNotFound("database not found by id: %d", id)
			}
			return err
		}

		if d.Lock != nil && d.Lock.InstanceID != 0 {
			return errdef.NewBadRequest("database already locked by user %q and instance %q", userId, d.Lock.InstanceID)
		}

		lock = &model.Lock{
			DatabaseID: id,
			InstanceID: instanceId,
			UserID:     userId,
		}
		return tx.Create(lock).Error
	})

	return lock, errTx
}

func (r repository) Unlock(id uint) error {
	err := r.db.Delete(&model.Lock{}, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errdef.NewNotFound("database not found by id: %d", id)
	}
	return err
}

func (r repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&model.Database{}, id).Error
}

func (r repository) FindByGroupNames(names []string) ([]model.Database, error) {
	var databases []model.Database

	err := r.db.
		Where("group_name IN ?", names).
		Find(&databases).Error

	return databases, err
}

func (r repository) Update(d *model.Database) error {
	return r.db.Save(d).Error
}

func (r repository) CreateExternalDownload(databaseID uint, expiration time.Time) (*model.ExternalDownload, error) {
	externalDownload := &model.ExternalDownload{
		UUID:       uuid.New(),
		Expiration: expiration,
		DatabaseID: databaseID,
	}

	err := r.db.Save(externalDownload).Error

	return externalDownload, err
}

func (r repository) FindExternalDownload(uuid uuid.UUID) (*model.ExternalDownload, error) {
	var d *model.ExternalDownload
	err := r.db.
		Where("expiration > ?", time.Now().UTC()).
		First(&d, uuid).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("external download not found by id: %s", uuid.String())
	}
	return d, err
}

func (r repository) PurgeExternalDownload() error {
	var d *model.ExternalDownload
	err := r.db.
		Where("expiration < ?", time.Now().UTC()).
		Delete(&d).Error
	return err
}
