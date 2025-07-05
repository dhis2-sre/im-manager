package model

import (
	"time"

	"github.com/google/uuid"
)

// swagger:model
type Database struct {
	ID                uint               `json:"id" gorm:"primaryKey"`
	CreatedAt         time.Time          `json:"createdAt"`
	UpdatedAt         time.Time          `json:"updatedAt"`
	Name              string             `json:"name" gorm:"index:database_name_group_idx,unique"`
	GroupName         string             `json:"groupName" gorm:"index:database_name_group_idx,unique"`
	Url               string             `json:"url"` // s3... Path?
	ExternalDownloads []ExternalDownload `json:"externalDownloads" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Lock              *Lock              `json:"lock" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Slug              string             `json:"slug" gorm:"uniqueIndex"`
	// TODO: Sql or fs?
	Type        string    `json:"type"`
	FilestoreID uint      `json:"filestoreId"`
	Filestore   *Database `json:"filestore" gorm:"foreignKey:ID"`
}

// swagger:model
type Lock struct {
	DatabaseID uint               `json:"databaseId" gorm:"primaryKey"`
	InstanceID uint               `json:"instanceId"`
	Instance   DeploymentInstance `json:"instance"`
	UserID     uint               `json:"userId"`
	User       User               `json:"user"`
}

// swagger:model
type ExternalDownload struct {
	UUID       uuid.UUID `json:"uuid" gorm:"primaryKey;type:uuid"`
	Expiration uint      `json:"expiration"`
	DatabaseID uint      `json:"databaseId"`
}
