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
	Description       string             `json:"description" gorm:"type:text"`
	Url               string             `json:"url"` // s3... Path?
	ExternalDownloads []ExternalDownload `json:"externalDownloads" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Lock              *Lock              `json:"lock" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Slug              string             `json:"slug" gorm:"uniqueIndex"`
	Type              string             `json:"type"` // TODO: Strictly sql or fs?
	FilestoreID       uint               `json:"filestoreId"`
	// Filestore is the file store saved alongside this database, resolved through FilestoreID.
	// Naming the foreign key explicitly matters here: the association points at another row of this
	// same table, and pointing it at ID instead resolves a database to itself. It is excluded from
	// migration on purpose: FilestoreID is a plain uint where 0 means "no file store", so an
	// enforced constraint would reject every database saved without one.
	Filestore *Database `json:"filestore" gorm:"foreignKey:FilestoreID;-:migration"`
	UserID    uint      `json:"userId"`
	User      User      `json:"user"`
	Size      int64     `json:"size"`
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
