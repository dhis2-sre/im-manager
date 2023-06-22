package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// swagger:model
type Database struct {
	gorm.Model
	Name              string             `gorm:"index:idx_name_and_group,unique"`
	GroupName         string             `gorm:"index:idx_name_and_group,unique"`
	Url               string             // s3... Path?
	ExternalDownloads []ExternalDownload `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Lock              *Lock              `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE; foreignKey:DatabaseID"`
	Slug              string             `gorm:"uniqueIndex"`
}

// swagger:model
type Lock struct {
	DatabaseID uint `gorm:"primaryKey"`
	InstanceID uint `gorm:"references:Instance"`
	UserID     uint `gorm:"references:User"`
}

// swagger:model
type ExternalDownload struct {
	UUID       uuid.UUID `gorm:"primaryKey;type:uuid"`
	Expiration uint
	DatabaseID uint
}
