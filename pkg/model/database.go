package model

import (
	"time"

	"github.com/google/uuid"
)

// swagger:model
type Database struct {
	ID                uint               `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time          `json:"createdAt"`
	UpdatedAt         time.Time          `json:"updatedAt"`
	Name              string             `gorm:"index:idx_name_and_group,unique" json:"name"`
	GroupName         string             `gorm:"index:idx_name_and_group,unique" json:"groupName"`
	Url               string             `json:"url"` // s3... Path?
	ExternalDownloads []ExternalDownload `json:"externalDownloads"`
	Lock              *Lock              `json:"lock"`
	Slug              string             `gorm:"uniqueIndex" json:"slug"`
}

// swagger:model GroupsWithDatabases
type GroupsWithDatabases struct {
	ID        uint       `json:"id"`
	Name      string     `json:"name"`
	Hostname  string     `json:"hostname"`
	Databases []Database `json:"databases"`
}

// swagger:model
type Lock struct {
	DatabaseID uint `gorm:"primaryKey" json:"databaseId"`
	InstanceID uint `json:"instanceId"`
	UserID     uint `json:"userId"`
}

// swagger:model
type ExternalDownload struct {
	UUID       uuid.UUID `gorm:"primaryKey;type:uuid" json:"uuid"`
	Expiration uint      `json:"expiration"`
	DatabaseID uint      `json:"databaseId"`
}
