package instance

import (
	"errors"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_service_Link(t *testing.T) {
	source := &model.Instance{}
	destination := &model.Instance{}

	repository := &mockInstanceRepository{}
	repository.
		On("Link", source, destination).
		Return(nil)

	service := NewService(config.Config{}, repository, nil, nil, nil)

	err := service.Link(source, destination)

	require.NoError(t, err)

	repository.AssertExpectations(t)
}

func Test_service_Link_RepositoryError(t *testing.T) {
	source := &model.Instance{}
	destination := &model.Instance{}

	errorMessage := "some error from the repository"
	repository := &mockInstanceRepository{}
	repository.
		On("Link", source, destination).
		Return(errors.New(errorMessage))

	service := NewService(config.Config{}, repository, nil, nil, nil)

	err := service.Link(source, destination)

	assert.ErrorContains(t, err, errorMessage)

	repository.AssertExpectations(t)
}

type mockInstanceRepository struct{ mock.Mock }

func (m *mockInstanceRepository) Link(firstInstance, secondInstance *model.Instance) error {
	return m.Called(firstInstance, secondInstance).Error(0)
}

func (m *mockInstanceRepository) Unlink(instance *model.Instance) error {
	panic("implement me")
}

func (m *mockInstanceRepository) Save(instance *model.Instance) error {
	panic("implement me")
}

func (m *mockInstanceRepository) FindById(id uint) (*model.Instance, error) {
	panic("implement me")
}

func (m *mockInstanceRepository) FindByIdDecrypted(id uint) (*model.Instance, error) {
	panic("implement me")
}

func (m *mockInstanceRepository) FindByNameAndGroup(instance string, group string) (*model.Instance, error) {
	panic("implement me")
}

func (m *mockInstanceRepository) FindByGroupNames(names []string, presets bool) ([]*model.Instance, error) {
	panic("implement me")
}

func (m *mockInstanceRepository) SaveDeployLog(instance *model.Instance, log string) error {
	panic("implement me")
}

func (m *mockInstanceRepository) Delete(id uint) error {
	panic("implement me")
}
