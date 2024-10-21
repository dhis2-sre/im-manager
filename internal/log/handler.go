// Package log provides slog handlers.
package log

import (
	"context"
	"log/slog"

	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// ContextHandler adds values from the [context.Context] to the [slog.Record]. [slog.Handler] is
// passed to [slog.Logger] which is then used throughout the app. It has to use the same attribute
// keys as the Gin [middleware.RequestLogger] so we can find logs created by the middleware and the
// [slog.Logger] context aware methods. As not every use of the logger will be within the context of
// an HTTP request it needs to be ok with keys not being set in the [context.Context].
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
	// logs outside of an HTTP request or a RabbitMQ TTL message
	if id, ok := middleware.GetCorrelationID(ctx); ok {
		r.AddAttrs(slog.String(middleware.RequestLoggerKeyCorrelationID, id))
	}

	// public HTTP routes do not have a user in the context
	if user, ok := model.GetUserFromContext(ctx); ok {
		r.AddAttrs(slog.Any(middleware.RequestLoggerKeyUser, user))
	}

	return rh.Handler.Handle(ctx, r)
}

func (rh *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return New(rh.Handler.WithAttrs(attrs))
}

func (rh *ContextHandler) WithGroup(name string) slog.Handler {
	return New(rh.Handler.WithGroup(name))
}
