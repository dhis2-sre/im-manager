package group

/*

import (
	"errors"
	"github.com/dhis2-sre/im-user/internal/errdef"
	"github.com/dhis2-sre/im-user/pkg/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_service_AddUser_GroupNotFound(t *testing.T) {
	var userId uint = 1
	groupName := "whatever"
	errorMessage := "also whatever"

	repository := &mockGroupRepository{}
	repository.
		On("find", groupName).
		Return(nil, errdef.NewNotFound(errors.New(errorMessage)))

	userService := &mockUserService{}

	service := NewService(repository, userService)

	err := service.AddUser(groupName, userId)

	assert.ErrorContains(t, err, errorMessage)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func Test_service_AddUser_UserNotFound(t *testing.T) {
	var userId uint = 1
	groupName := "whatever"
	errorMessage := "also whatever"

	repository := &mockGroupRepository{}
	repository.
		On("find", groupName).
		Return(&model.Group{Name: groupName}, nil)

	userService := &mockUserService{}
	userService.
		On("FindById", userId).
		Return(nil, errdef.NewNotFound(errors.New(errorMessage)))

	service := NewService(repository, userService)

	err := service.AddUser(groupName, userId)

	assert.ErrorContains(t, err, errorMessage)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

import (
	"errors"
	"testing"

	"github.com/dhis2-sre/im-user/internal/errdef"
	"github.com/dhis2-sre/im-user/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func Test_service_GetClusterConfiguration_Happy(t *testing.T) {
	var id uint = 1
	groupName := "whatever"
	clusterConfiguration := &model.ClusterConfiguration{
		Model:                   gorm.Model{ID: id},
		GroupName:               groupName,
		KubernetesConfiguration: nil,
	}

	repository := &mockGroupRepository{}
	repository.
		On("getClusterConfiguration", groupName).
		Return(clusterConfiguration, nil)
	userService := &mockUserService{}

	service := NewService(repository, userService)

	cc, err := service.GetClusterConfiguration(groupName)

	require.NoError(t, err)
	assert.Equal(t, id, cc.ID)
	assert.Equal(t, groupName, cc.GroupName)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func Test_service_AddClusterConfiguration_Happy(t *testing.T) {
	clusterConfiguration := &model.ClusterConfiguration{}
	repository := &mockGroupRepository{}
	repository.On("addClusterConfiguration", clusterConfiguration).Return(nil)

	userService := &mockUserService{}

	service := NewService(repository, userService)

	err := service.AddClusterConfiguration(clusterConfiguration)

	require.NoError(t, err)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func Test_service_AddUser_Happy(t *testing.T) {
	var userId uint = 1
	groupName := "whatever"
	group := &model.Group{Name: groupName}
	user := &model.User{
		Model: gorm.Model{ID: userId},
	}

	repository := &mockGroupRepository{}
	repository.
		On("find", groupName).
		Return(group, nil)
	repository.
		On("addUser", group, user).
		Return(nil)

	userService := &mockUserService{}
	userService.
		On("FindById", userId).
		Return(user, nil)

	service := NewService(repository, userService)

	err := service.AddUser(groupName, userId)

	require.NoError(t, err)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func Test_service_Create_Happy(t *testing.T) {
	name := "whatever"
	hostname := "also whatever"

	repository := &mockGroupRepository{}
	repository.
		On("create", &model.Group{Name: name, Hostname: hostname}).
		Return(&model.Group{
			Name:     name,
			Hostname: hostname,
		}, nil)

	userService := &mockUserService{}

	service := NewService(repository, userService)

	found, err := service.Create(name, hostname)

	require.NoError(t, err)
	assert.Equal(t, name, found.Name)
	assert.Equal(t, hostname, found.Hostname)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func Test_service_Create_NotFound(t *testing.T) {
	name := "whatever"
	hostname := "also whatever"
	errorMessage := "not found"

	repository := &mockGroupRepository{}
	repository.
		On("create", &model.Group{Name: name, Hostname: hostname}).
		Return(nil, errdef.NewNotFound(errors.New(errorMessage)))

	userService := &mockUserService{}

	service := NewService(repository, userService)

	found, err := service.Create(name, hostname)

	assert.True(t, found == nil)
	assert.ErrorContains(t, err, errorMessage)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func Test_service_Find_Happy(t *testing.T) {
	name := "whatever"

	repository := &mockGroupRepository{}
	repository.
		On("find", name).
		Return(&model.Group{
			Name: name,
		}, nil)

	userService := &mockUserService{}

	service := NewService(repository, userService)

	found, err := service.Find(name)

	require.NoError(t, err)
	assert.Equal(t, name, found.Name)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func Test_service_Find_NotFound(t *testing.T) {
	name := "whatever"
	errorMessage := "not found"

	repository := &mockGroupRepository{}
	repository.
		On("find", name).
		Return(nil, errdef.NewNotFound(errors.New(errorMessage)))

	userService := &mockUserService{}

	service := NewService(repository, userService)

	found, err := service.Find(name)

	assert.True(t, found == nil)
	assert.ErrorContains(t, err, errorMessage)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func Test_service_FindOrCreate_Happy(t *testing.T) {
	name := "whatever"
	hostname := "also whatever"

	repository := &mockGroupRepository{}
	group := &model.Group{Name: name, Hostname: hostname}
	repository.
		On("findOrCreate", group).
		Return(group, nil)

	userService := &mockUserService{}

	service := NewService(repository, userService)

	found, err := service.FindOrCreate(name, hostname)

	require.NoError(t, err)
	assert.Equal(t, name, found.Name)
	assert.Equal(t, hostname, found.Hostname)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func Test_service_FindOrCreate_NotFound(t *testing.T) {
	name := "whatever"
	hostname := "also whatever"
	errorMessage := "not found"

	repository := &mockGroupRepository{}
	group := &model.Group{Name: name, Hostname: hostname}
	repository.
		On("findOrCreate", group).
		Return(nil, errdef.NewNotFound(errors.New(errorMessage)))

	userService := &mockUserService{}

	service := NewService(repository, userService)

	found, err := service.FindOrCreate(name, hostname)

	assert.True(t, found == nil)
	assert.ErrorContains(t, err, errorMessage)

	userService.AssertExpectations(t)
	repository.AssertExpectations(t)
}

type mockGroupRepository struct{ mock.Mock }

func (m *mockGroupRepository) create(group *model.Group) error {
	called := m.Called(group)
	_, ok := called.Get(0).(*model.Group)
	if ok {
		return nil
	} else {
		return called.Error(1)
	}
}

func (m *mockGroupRepository) addUser(group *model.Group, user *model.User) error {
	called := m.Called(group, user)
	return called.Error(0)
}

func (m *mockGroupRepository) addClusterConfiguration(configuration *model.ClusterConfiguration) error {
	called := m.Called(configuration)
	return called.Error(0)
}

func (m *mockGroupRepository) getClusterConfiguration(groupName string) (*model.ClusterConfiguration, error) {
	called := m.Called(groupName)
	configuration, ok := called.Get(0).(*model.ClusterConfiguration)
	if ok {
		return configuration, nil
	} else {
		return nil, called.Error(1)
	}
}

func (m *mockGroupRepository) find(name string) (*model.Group, error) {
	called := m.Called(name)
	group, ok := called.Get(0).(*model.Group)
	if ok {
		return group, nil
	} else {
		return nil, called.Error(1)
	}
}

func (m *mockGroupRepository) findOrCreate(group *model.Group) (*model.Group, error) {
	called := m.Called(group)
	group, ok := called.Get(0).(*model.Group)
	if ok {
		return group, nil
	} else {
		return nil, called.Error(1)
	}
}

type mockUserService struct{ mock.Mock }

func (m *mockUserService) FindById(id uint) (*model.User, error) {
	called := m.Called(id)
	user, ok := called.Get(0).(*model.User)
	if ok {
		return user, nil
	} else {
		return nil, called.Error(1)
	}
}
*/
