package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ctxKey int

var correlationIDKey ctxKey

// CorrelationID is a Gin middleware that adds a generated correlation ID to the
// [http.Request.Context].
func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = NewContextWithCorrelationID(ctx, uuid.NewString())
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// NewContextWithCorrelationID returns a new [context.Context] that carries value correlationID.
func NewContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// GetCorrelationID returns the correlation ID stored in the ctx, if any. It had to have been set by
// the [CorrelationID] middleware before.
func GetCorrelationID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(correlationIDKey).(string)
	return id, ok
}

// RequestLogger logs details like request time, response time, latency and more about every
// request.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestTime := time.Now()

		c.Next()

		responseTime := time.Now()

		params := make(map[string]string, len(c.Params))
		for _, param := range c.Params {
			params[param.Key] = param.Value
		}
		requestAttribute := slog.Group("request",
			slog.Time("time", requestTime),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("route", c.FullPath()),
			slog.String("query", c.Request.URL.RawQuery),
			slog.Any("params", params),
			slog.String("host", c.Request.Host),
			slog.String("userAgent", c.Request.UserAgent()),
			slog.String("ip", c.ClientIP()),
		)
		responseAttribute := slog.Group("response",
			slog.Time("time", responseTime),
			slog.Duration("latency", responseTime.Sub(requestTime)),
			slog.Int("status", c.Writer.Status()),
		)

		level := slog.LevelInfo
		const msg = "Processed HTTP request"
		var errorAttribute slog.Attr
		if status := c.Writer.Status(); status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			level = slog.LevelWarn
			errorAttribute = slog.String("error", c.Errors.String())
		} else if status >= http.StatusInternalServerError {
			level = slog.LevelError
			errorAttribute = slog.String("error", c.Errors.String())
		}

		logger.LogAttrs(c.Request.Context(), level, msg, errorAttribute, requestAttribute, responseAttribute)
	}
}
