package database

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func Test_service_FindByIdentifier_Slug(t *testing.T) {
	repository := &mockRepository{}
	database := &model.Database{Name: "database name"}
	repository.
		On("FindBySlug", "database-identifier").
		Return(database, nil)
	service := NewService(Config{}, nil, nil, repository)

	d, err := service.FindByIdentifier("database-identifier")

	require.NoError(t, err)
	require.Equal(t, database.Name, d.Name)
	repository.AssertExpectations(t)
}

func Test_service_FindByIdentifier_Slug_NotFound(t *testing.T) {
	repository := &mockRepository{}
	repository.
		On("FindBySlug", "database-identifier").
		Return(nil, gorm.ErrRecordNotFound)
	service := NewService(Config{}, nil, nil, repository)

	d, err := service.FindByIdentifier("database-identifier")

	require.Nil(t, d)
	require.ErrorContains(t, err, "database not found by slug with value: database-identifier not found")
}

func Test_service_FindByIdentifier_Id(t *testing.T) {
	repository := &mockRepository{}
	database := &model.Database{Name: "database name"}
	repository.
		On("FindById", uint(1)).
		Return(database, nil)
	service := NewService(Config{}, nil, nil, repository)

	d, err := service.FindByIdentifier("1")

	require.NoError(t, err)
	require.Equal(t, database.Name, d.Name)
	repository.AssertExpectations(t)
}

func Test_service_FindByIdentifier_Id_NotFound(t *testing.T) {
	repository := &mockRepository{}
	repository.
		On("FindById", uint(1)).
		Return(nil, gorm.ErrRecordNotFound)
	service := NewService(Config{}, nil, nil, repository)

	d, err := service.FindByIdentifier("1")

	require.Nil(t, d)
	require.ErrorContains(t, err, "database not found by id with value: 1 not found")
}
