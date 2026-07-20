package deployment

import "github.com/dhis2-sre/im-manager/pkg/model"

const kindFilestoreBackup = "filestore-backup"

// filestoreEvent is the JSON payload published for filestore-backup events. It matches the wire
// format these events had when they were published from the database package.
type filestoreEvent struct {
	Status       string `json:"status"`
	DatabaseID   uint   `json:"databaseId"`
	DatabaseName string `json:"databaseName"`
	Error        string `json:"error,omitempty"`
}

func newFilestoreEvent(db *model.Database, status, errMsg string) filestoreEvent {
	return filestoreEvent{
		Status:       status,
		DatabaseID:   db.ID,
		DatabaseName: db.Name,
		Error:        errMsg,
	}
}
