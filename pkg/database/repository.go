package database

import (
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

	err := r.db.Save(&d).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("database named %q already exists", d.Name)
	}

	return err
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

func (r repository) UpdateId(old, new uint) error {
	return r.db.
		Model(&model.Database{}).
		Where("id = ?", old).
		Update("id", new).
		Error
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

func (r repository) Lock(databaseId, instanceId, userId uint) (*model.Lock, error) {
	var lock *model.Lock

	errTx := r.db.Transaction(func(tx *gorm.DB) error {
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

func (r repository) Unlock(databaseId uint) error {
	db := r.db.Unscoped().Delete(&model.Lock{}, "database_id = ?", databaseId)
	if db.Error != nil {
		return db.Error
	}

	if db.RowsAffected < 1 {
		return errdef.NewNotFound("lock not found by database id: %d", databaseId)
	}

	return nil
}

func (r repository) Delete(id uint) error {
	database, err := r.FindById(id)
	if err != nil {
		return err
	}

	if database.Lock != nil {
		return errdef.NewBadRequest("database is locked")
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
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

func (r repository) FindByGroupNames(groupNames []string) ([]model.Database, error) {
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

func (r repository) Update(d *model.Database) error {
	return r.db.Save(d).Error
}

func (r repository) CreateExternalDownload(databaseID uint, expirationInSeconds uint) (*model.ExternalDownload, error) {
	externalDownload := &model.ExternalDownload{
		UUID:       uuid.New(),
		Expiration: uint(time.Now().Unix()) + expirationInSeconds,
		DatabaseID: databaseID,
	}

	err := r.db.Save(externalDownload).Error

	return externalDownload, err
}

func (r repository) FindExternalDownload(uuid uuid.UUID) (*model.ExternalDownload, error) {
	var d *model.ExternalDownload
	err := r.db.
		Where("expiration > ?", time.Now().Unix()).
		First(&d, uuid).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("external download not found by id: %q", uuid)
	}
	return d, err
}

func (r repository) PurgeExternalDownload() error {
	var d *model.ExternalDownload
	err := r.db.
		Where("expiration < ?", time.Now().Unix()).
		Delete(&d).Error
	return err
}
