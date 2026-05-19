package model

import "time"

type Notification struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UserID    uint      `json:"userId"`
	GroupName string    `json:"groupName"`
	Kind      string    `json:"kind"`
	Data      string    `json:"data" gorm:"type:text"`
	Read      bool      `json:"read"`
}
