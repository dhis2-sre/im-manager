package model

import "time"

type Event struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	Kind      string    `json:"kind"`
	GroupName string    `json:"groupName" gorm:"references:Name"`
	Group     Group     `json:"group"`
	UserID    *uint     `json:"userId"`
	User      *User     `json:"-"`
	Payload   any       `json:"payload" gorm:"type:jsonb;default:'[]';not null"`
}
