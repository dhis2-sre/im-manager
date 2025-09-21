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
	Description       string             `json:"description" gorm:"type:text;index:idx_database_description,type:gin,opclass:gin_trgm_ops"`
	Url               string             `json:"url"` // s3... Path?
	ExternalDownloads []ExternalDownload `json:"externalDownloads" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Lock              *Lock              `json:"lock" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Slug              string             `json:"slug" gorm:"uniqueIndex"`
	Type              string             `json:"type"` // TODO: Strictly sql or fs?
	FilestoreID       uint               `json:"filestoreId"`
	Filestore         *Database          `json:"filestore" gorm:"foreignKey:ID"`
	UserID            uint               `json:"userId"`
	User              User               `json:"user"`
}

// swagger:model
type Lock struct {
	DatabaseID uint               `json:"databaseId" gorm:"primaryKey"`
	InstanceID uint               `json:"instanceId"`
	Instance   DeploymentInstance `json:"instance,omitempty"`
	UserID     uint               `json:"userId"`
	User       User               `json:"user,omitempty"`
}

// swagger:model
type ExternalDownload struct {
	UUID       uuid.UUID `json:"uuid" gorm:"primaryKey;type:uuid"`
	Expiration uint      `json:"expiration"`
	DatabaseID uint      `json:"databaseId"`
}
