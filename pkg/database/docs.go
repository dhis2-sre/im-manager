package database

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// swagger:response
type Error struct {
	// The error message
	//in: body
	Message string
}

//swagger:parameters findDatabase lockDatabaseById unlockDatabaseById downloadDatabase deleteDatabaseById updateDatabaseById createExternalDownloadDatabase
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}

//swagger:parameters saveAsDatabase
type _ struct {
	// in: path
	// required: true
	InstanceID uint `json:"instanceId"`

	// SaveAs database request body parameter
	// in: body
	// required: true
	Body saveAsRequest
}

//swagger:parameters saveDatabase
type _ struct {
	// in: path
	// required: true
	InstanceID uint `json:"instanceId"`
}

// swagger:parameters copyDatabase
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`

	// Copy database request body parameter
	// in: body
	// required: true
	Body CopyDatabaseRequest
}

// swagger:parameters lockDatabaseById unlockDatabaseById
type _ struct {
	// Lock/unlock database request body parameter
	// in: body
	// required: true
	Body LockDatabaseRequest
}

// swagger:parameters uploadDatabase
type uploadDatabaseParams struct {
	// Required custom header representing the name of the file
	// in: header
	// required: true
	Name string `json:"X-Upload-Name"`

	// Required custom header representing the group name
	// in: header
	// required: true
	Group string `json:"X-Upload-Group"`

	// The file content
	// in: body
	// required: true
	Body []byte
}

// swagger:parameters updateDatabaseById
type _ struct {
	// Update database request body parameter
	// in: body
	// required: true
	Body UpdateDatabaseRequest
}

//swagger:parameters externalDownloadDatabase
type _ struct {
	// in: path
	// required: true
	// swagger:strfmt uuid
	UUID string `json:"uuid"`
}

// swagger:response DownloadDatabaseResponse
type DownloadDatabaseBody struct {
	//in: body
	Body []byte
}

// swagger:parameters createExternalDownloadDatabase
type _ struct {
	// Create external database download
	// in: body
	// required: true
	Body CreateExternalDatabaseRequest
}

// swagger:response CreateExternalDownloadResponse
type CreateExternalDownloadBody struct {
	//in: body
	Body model.ExternalDownload
}

// swagger:response Database
type DatabaseBody struct {
	//in: body
	Body model.Database
}

// swagger:response GroupsWithDatabases
type GroupsWithDatabasesBody struct {
	//in: body
	Body GroupsWithDatabases
}

// swagger:response Lock
type LockBody struct {
	//in: body
	Body model.Lock
}
