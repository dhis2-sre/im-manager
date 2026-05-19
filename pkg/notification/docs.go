package notification

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// swagger:response Notification
type _ struct {
	// in: body
	_ []model.Notification
}

// swagger:parameters markNotificationRead
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}
