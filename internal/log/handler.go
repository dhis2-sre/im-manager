// Package log provides slog handlers.
package log

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

// RequestIDHandler adds the request ID to the slog.Record if present in the Gin context.
type RequestIDHandler struct {
	slog.Handler
	group string
}

func New(handler slog.Handler, group string) RequestIDHandler {
	return RequestIDHandler{
		Handler: handler,
		group:   group,
	}
}

func (rh RequestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if ginCtx, ok := ctx.(*gin.Context); ok {
		// we are logging the request id under the same key used by the slog Gin middleware
		// https://github.com/samber/slog-gin/blob/812b3ffb5d6c562fa79e00edaef5409cd053f4d0/middleware.go#L167-L169
		// this allows us to find logs we make via the context aware logger functions like
		// logger.InfoContext as well as the ones made by the Gin middleware using the same key and
		// id
		r.AddAttrs(slog.Group(rh.group, slog.String("id", sloggin.GetRequestID(ginCtx))))
	}
	return rh.Handler.Handle(ctx, r)
}
