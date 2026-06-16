package notification

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// swagger:response Notification
type NotificationBody struct {
	// in: body
	Body []model.Notification
}

// swagger:parameters markNotificationRead
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}
