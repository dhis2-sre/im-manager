package database

import "github.com/dhis2-sre/im-manager/pkg/model"

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

func newDatabaseEvent(db *model.Database, status, errMsg string, size int64) databaseEvent {
	return databaseEvent{
		Status:       status,
		DatabaseID:   db.ID,
		DatabaseName: db.Name,
		Size:         size,
		Error:        errMsg,
	}
}
