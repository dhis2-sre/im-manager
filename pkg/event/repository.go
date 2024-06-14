package event

import (
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

func (r repository) Create(event model.Event) error {
	return r.db.Create(&event).Error
}
