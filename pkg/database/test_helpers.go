package database

// Test helpers used across packages. Kept in the main package so they can be
// reused by external tests that depend on database models and handlers.

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func CreateDatabaseRecord(t *testing.T, db *gorm.DB, name, groupName, url string, userID uint) *model.Database {
	t.Helper()

	database := &model.Database{
		Name:      name,
		GroupName: groupName,
		Type:      "database",
		Url:       url,
		UserID:    userID,
	}

	err := db.Create(database).Error
	require.NoError(t, err, "failed to create test database record")

	return database
}

func UploadTestDatabase(t *testing.T, client *inttest.HTTPClient, name, content, group string, headers ...func(http.Header)) string {
	t.Helper()

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	err := writer.WriteField("group", group)
	require.NoError(t, err, "failed to write group form field")

	err = writer.WriteField("name", name)
	require.NoError(t, err, "failed to write name form field")

	file, err := writer.CreateFormFile("database", "database")
	require.NoError(t, err, "failed to create form file")

	_, err = file.Write([]byte(content))
	require.NoError(t, err, "failed to write file content")

	err = writer.Close()
	require.NoError(t, err, "failed to close multipart writer")

	baseHeaders := []func(http.Header){
		inttest.WithHeader("X-Upload-Group", group),
		inttest.WithHeader("X-Upload-Name", name),
		inttest.WithHeader("X-Upload-Description", "Test database"),
		inttest.WithHeader("Content-Type", writer.FormDataContentType()),
	}

	if len(headers) > 0 {
		baseHeaders = append(baseHeaders, headers...)
	}

	body := client.Put(t, "/databases", &buffer, http.StatusCreated, baseHeaders...)

	var actualDB model.Database
	err = json.Unmarshal(body, &actualDB)
	require.NoError(t, err, "failed to unmarshal database response")

	return strconv.FormatUint(uint64(actualDB.ID), 10)
}
