package event

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB) Repository {
	return Repository{db}
}

type Repository struct {
	db *gorm.DB
}

func (r Repository) Create(event model.Event) error {
	return r.db.Create(&event).Error
}
