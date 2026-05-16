package database

import (
	"context"
	"log/slog"
)

const (
	kindDatabaseSave    = "database-save"
	kindFilestoreBackup = "filestore-backup"
)

// databaseEvent is the JSON payload published for database-save and filestore-backup events.
// Size is omitted (omitempty) for filestore-backup, which doesn't produce one.
type databaseEvent struct {
	Status       string `json:"status"`
	DatabaseID   uint   `json:"databaseId"`
	DatabaseName string `json:"databaseName"`
	Size         int64  `json:"size,omitempty"`
	Error        string `json:"error,omitempty"`
}

// publishEvent is fire-and-forget: a nil publisher is a no-op, errors are logged.
// JSON marshaling is the Publisher's responsibility.
func publishEvent(ctx context.Context, logger *slog.Logger, publisher Publisher, userID uint, groupName, kind string, payload databaseEvent) {
	if publisher == nil {
		return
	}
	if err := publisher.Publish(ctx, userID, groupName, kind, payload); err != nil {
		logger.ErrorContext(ctx, "failed to publish event", "kind", kind, "error", err)
	}
}
