package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// gin.Context only allows string keys so the best we can do is to use a prefix to avoid name
// clashes
const requestIDKey = "im-manager.requestID"

const (
	// slog key under which to log the request id
	RequestLoggerKeyID = "id"
	// slog key under which to log the request user
	RequestLoggerKeyUser = "user"
)

// RequestID adds a request ID to each request.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(requestIDKey, uuid.NewString())

		c.Next()
	}
}

// GetRequestID gets the request ID added by the RequestID middleware out of the request context.
func GetRequestID(c *gin.Context) string {
	v, ok := c.Get(requestIDKey)
	if !ok {
		return ""
	}

	ID, ok := v.(string)
	if !ok {
		return ""
	}

	return ID
}

// RequestLogger logs details like request time, response time, latency and more about every
// request.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestTime := time.Now()

		c.Next()

		responseTime := time.Now()

		idAttribute := slog.String(RequestLoggerKeyID, GetRequestID(c))
		var userAttribute slog.Attr
		if ctxUser, ok := c.Get("user"); ok {
			if user, ok := ctxUser.(*model.User); ok {
				userAttribute = slog.Any(RequestLoggerKeyUser, user)
			}
		}
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

		msg := "Processed HTTP request"
		level := slog.LevelInfo
		var errorAttribute slog.Attr
		if status := c.Writer.Status(); status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			level = slog.LevelWarn
			errorAttribute = slog.String("error", c.Errors.String())
		} else if status >= http.StatusInternalServerError {
			level = slog.LevelError
			errorAttribute = slog.String("error", c.Errors.String())
		}

		logger.LogAttrs(c.Request.Context(), level, msg, idAttribute, userAttribute,
			errorAttribute, requestAttribute, responseAttribute)
	}
}
