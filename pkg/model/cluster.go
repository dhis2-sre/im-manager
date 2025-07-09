package model

import "time"

type Cluster struct {
	// required: true
	ID uint `json:"id" gorm:"primaryKey"`
	// required: true
	CreatedAt time.Time `json:"createdAt"`
	// required: true
	UpdatedAt time.Time `json:"updatedAt"`
	// required: true
	Name string `json:"name" gorm:"uniqueIndex"`
	// required: true
	Description   string  `json:"description"`
	Configuration []byte  `json:"-"`
	Groups        []Group `json:"groups,omitempty" gorm:"constraint:OnUpdate:CASCADE"`
}
