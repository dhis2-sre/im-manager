package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// swagger:model
type Database struct {
	gorm.Model
	Name              string `gorm:"index:idx_name_and_group,unique"`
	GroupName         string `gorm:"index:idx_name_and_group,unique"`
	Url               string // s3... Path?
	ExternalDownloads []ExternalDownload
	Lock              *Lock
	Slug              string `gorm:"uniqueIndex"`
}

// swagger:model
type Lock struct {
	DatabaseID uint `gorm:"primaryKey"`
	InstanceID uint
	UserID     uint
}

// swagger:model
type ExternalDownload struct {
	UUID       uuid.UUID `gorm:"primaryKey;type:uuid"`
	Expiration time.Time
	DatabaseID uint
}
