// Package log provides slog handlers.
package log

import (
	"context"
	"log/slog"

	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

// ContextHandler adds values from the Gin context to the slog.Record.
type ContextHandler struct {
	slog.Handler
}

func New(handler slog.Handler) *ContextHandler {
	return &ContextHandler{
		Handler: handler,
	}
}

func (rh *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return rh.Handler.Enabled(ctx, level)
}

func (rh *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// We are logging the request id/user under the same keys as in middleware.RequestLogger. This
	// is to find logs we make via the context aware logger functions like logger.InfoContext as
	// well as the ones made by the Gin middleware.RequestLogger.
	if ginCtx, ok := ctx.(*gin.Context); ok {
		r.AddAttrs(slog.String(middleware.RequestLoggerKeyID, middleware.GetRequestID(ginCtx)))

		if user, ok := ginCtx.Get("user"); ok {
			if user, ok := user.(*model.User); ok {
				r.AddAttrs(slog.Any(middleware.RequestLoggerKeyUser, user))
			}
		}
	}
	return rh.Handler.Handle(ctx, r)
}

func (rh *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return New(rh.Handler.WithAttrs(attrs))
}

func (rh *ContextHandler) WithGroup(name string) slog.Handler {
	return New(rh.Handler.WithGroup(name))
}
