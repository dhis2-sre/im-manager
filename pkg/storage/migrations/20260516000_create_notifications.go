package migrations

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func createNotifications() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260516000",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.Notification{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&model.Notification{})
		},
	}
}
