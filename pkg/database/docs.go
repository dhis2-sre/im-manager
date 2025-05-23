package database

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"mime/multipart"
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
	// Required custom header: X-Upload-Name
	// in: header
	// required: true
	Name string `json:"name"`

	// Required custom header: X-Upload-Group
	// in: header
	// required: true
	Group string `json:"group"`

	// The file to upload
	// in: body
	// required: true
	Body *multipart.FileHeader
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
	UUID uint `json:"uuid"`
}

// swagger:response DownloadDatabaseResponse
type _ struct {
	//in: body
	_ []byte
}

// swagger:parameters createExternalDownloadDatabase
type _ struct {
	// Create external database download
	// in: body
	// required: true
	Body CreateExternalDatabaseRequest
}

// swagger:response CreateExternalDownloadResponse
type _ struct {
	//in: body
	_ model.ExternalDownload
}

// swagger:response Database
type _ struct {
	//in: body
	_ model.Database
}

// swagger:response GroupsWithDatabases
type _ struct {
	//in: body
	_ GroupsWithDatabases
}

// swagger:response Lock
type _ struct {
	//in: body
	_ model.Lock
}
