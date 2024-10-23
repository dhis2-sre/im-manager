// Package log provides slog handlers.
package log

import (
	"context"
	"log/slog"

	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

const (
	// loggerKeyCorrelationID is the slog key under which to log the correlation id
	loggerKeyCorrelationID = "correlationId"
	// loggerKeyUser is the slog key under which to log the user
	loggerKeyUser = "user"
)

// ContextHandler adds values from the [context.Context] to the [slog.Record]. [slog.Handler] is
// passed to [slog.Logger] which is then used throughout the app.
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
	// logs outside of an HTTP request or a RabbitMQ TTL message might not have a correlationID
	if id, ok := middleware.GetCorrelationID(ctx); ok {
		r.AddAttrs(slog.String(loggerKeyCorrelationID, id))
	}

	// public HTTP routes do not have a user in the context
	if user, ok := model.GetUserFromContext(ctx); ok {
		r.AddAttrs(slog.Any(loggerKeyUser, user))
	}

	return rh.Handler.Handle(ctx, r)
}

func (rh *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return New(rh.Handler.WithAttrs(attrs))
}

func (rh *ContextHandler) WithGroup(name string) slog.Handler {
	return New(rh.Handler.WithGroup(name))
}
