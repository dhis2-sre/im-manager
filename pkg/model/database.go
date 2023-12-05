package model

import (
	"time"

	"github.com/google/uuid"
)

// swagger:model
type Database struct {
	ID                uint               `json:"id" gorm:"primarykey"`
	CreatedAt         time.Time          `json:"createdAt"`
	UpdatedAt         time.Time          `json:"updatedAt"`
	Name              string             `json:"name" gorm:"index:idx_name_and_group,unique"`
	GroupName         string             `json:"groupName" gorm:"index:idx_name_and_group,unique"`
	Url               string             `json:"url"` // s3... Path?
	ExternalDownloads []ExternalDownload `json:"externalDownloads" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Lock              *Lock              `json:"lock" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Slug              string             `json:"slug" gorm:"uniqueIndex"`
}

// swagger:model
type Lock struct {
	DatabaseID uint     `json:"databaseId" gorm:"primaryKey"`
	InstanceID uint     `json:"instanceId"`
	Instance   Instance `json:"-"`
	UserID     uint     `json:"userId"`
	User       User     `json:"-"`
}

// swagger:model
type ExternalDownload struct {
	UUID       uuid.UUID `json:"uuid" gorm:"primaryKey;type:uuid"`
	Expiration uint      `json:"expiration"`
	DatabaseID uint      `json:"databaseId"`
}
