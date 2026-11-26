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
	// SubjectExists reports whether the deployment or database the notification is about still
	// exists and the requesting user may open it. It is left out for a kind without such a subject.
	SubjectExists *bool `json:"subjectExists,omitempty" gorm:"-"`
}
