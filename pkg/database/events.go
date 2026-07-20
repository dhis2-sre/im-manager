package database

import "github.com/dhis2-sre/im-manager/pkg/model"

const kindDatabaseSave = "database-save"

// databaseEvent is the JSON payload published for database-save events.
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
