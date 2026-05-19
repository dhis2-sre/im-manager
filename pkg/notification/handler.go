package notification

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

type notificationService interface {
	FindByUserID(ctx context.Context, userID uint) ([]model.Notification, error)
	MarkRead(ctx context.Context, id, userID uint) error
	MarkAllRead(ctx context.Context, userID uint) error
}

func NewHandler(logger *slog.Logger, service notificationService) Handler {
	return Handler{logger: logger, service: service}
}

type Handler struct {
	logger  *slog.Logger
	service notificationService
}

// List notifications
func (h Handler) List(c *gin.Context) {
	// swagger:route GET /notifications listNotifications
	//
	// List notifications
	//
	// List notifications for the authenticated user, ordered newest first.
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	200: []Notification
	//	401: Error
	//	415: Error
	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	notifications, err := h.service.FindByUserID(ctx, user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, notifications)
}

// MarkRead marks a notification as read
func (h Handler) MarkRead(c *gin.Context) {
	// swagger:route PUT /notifications/{id}/read markNotificationRead
	//
	// Mark notification as read
	//
	// Mark a notification as read by its id.
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	204:
	//	401: Error
	//	404: Error
	//	415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.service.MarkRead(ctx, id, user.ID); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}

// MarkAllRead marks all notifications as read
func (h Handler) MarkAllRead(c *gin.Context) {
	// swagger:route PUT /notifications/read-all markAllNotificationsRead
	//
	// Mark all notifications as read
	//
	// Mark all notifications as read for the authenticated user.
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	204:
	//	401: Error
	//	415: Error
	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.service.MarkAllRead(ctx, user.ID); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}
