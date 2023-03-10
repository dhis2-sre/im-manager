package instance

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/config"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHandler_Deploy(t *testing.T) {
	userClient := &mockUserClient{}
	group := &models.Group{
		Name:                 "group name",
		ClusterConfiguration: &models.ClusterConfiguration{},
	}
	userClient.
		On("FindGroupByName", "token", "group name").
		Return(group, nil)
	helmfileService := &mockHelmfileService{}
	instance := &model.Instance{
		Name:      "instance-name",
		GroupName: "group name",
		StackName: "instance stack",
		UserID:    1,
	}
	helmfileService.
		On("sync", "token", instance, group).
		Return(exec.Command("echo", "-n", ""), nil)
	repository := &mockRepository{}
	repository.
		On("FindByIdDecrypted", uint(0)).
		Return(instance, nil)
	repository.
		On("Save", instance).
		Return(nil)
	repository.
		On("SaveDeployLog", instance, "").
		Return(nil)
	service := NewService(config.Config{}, repository, userClient, nil, helmfileService)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")
	c.AddParam("id", "1")
	c.Request = newPost(t, "", &DeployInstanceRequest{
		Name:  "instance-name",
		Group: "group name",
		Stack: "instance stack",
	})

	handler.Deploy(c)

	require.Empty(t, c.Errors)
	require.Equal(t, http.StatusCreated, w.Code)
	repository.AssertExpectations(t)
}

func TestHandler_Update(t *testing.T) {
	userClient := &mockUserClient{}
	group := &models.Group{
		Name:                 "group name",
		ClusterConfiguration: &models.ClusterConfiguration{},
	}
	userClient.
		On("FindGroupByName", "token", "group name").
		Return(group, nil)
	helmfileService := &mockHelmfileService{}
	instance := &model.Instance{
		Model:     gorm.Model{ID: 1},
		UserID:    1,
		Name:      "instance-name",
		GroupName: "group name",
		StackName: "instance stack",
	}
	helmfileService.
		On("sync", "token", instance, group).
		Return(exec.Command("echo", "-n", ""), nil)
	repository := &mockRepository{}
	repository.
		On("FindById", uint(1)).
		Return(instance, nil)
	repository.
		On("FindByIdDecrypted", uint(1)).
		Return(instance, nil)
	repository.
		On("Save", instance).
		Return(nil)
	repository.
		On("SaveDeployLog", instance, "").
		Return(nil)
	service := NewService(config.Config{}, repository, userClient, nil, helmfileService)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")
	c.AddParam("id", "1")
	c.Request = newPost(t, "", &UpdateInstanceRequest{
		RequiredParameters: nil,
		OptionalParameters: nil,
	})

	handler.Update(c)

	require.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusAccepted, instance)
	repository.AssertExpectations(t)
}

func newPost(t *testing.T, path string, jsonBody any) *http.Request {
	body, err := json.Marshal(jsonBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", "token")

	return req
}

func TestHandler_ListInstances(t *testing.T) {
	repository := &mockRepository{}
	groups := []*models.Group{
		{Name: "group name"},
	}
	groupsWithInstances := []GroupWithInstances{
		{
			Name: "group name",
			Instances: []*model.Instance{
				{
					Model:     gorm.Model{ID: 1},
					Name:      "instance name",
					GroupName: "group name",
				},
			},
		},
	}
	repository.
		On("FindByGroups", groups, false).
		Return(groupsWithInstances, nil)
	service := NewService(config.Config{}, repository, nil, nil, nil)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")

	handler.ListInstances(c)

	require.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusOK, groupsWithInstances)
	repository.AssertExpectations(t)
}

func TestHandler_ListInstances_RepositoryError(t *testing.T) {
	groups := []*models.Group{
		{Name: "group name"},
	}
	repository := &mockRepository{}
	repository.
		On("FindByGroups", groups, false).
		Return(nil, errors.New("some error"))
	service := NewService(config.Config{}, repository, nil, nil, nil)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")

	handler.ListInstances(c)

	require.Empty(t, w.Body.Bytes())
	require.Len(t, c.Errors, 1)
	require.ErrorContains(t, c.Errors[0].Err, "some error")
	repository.AssertExpectations(t)
}

func TestHandler_ListPresets(t *testing.T) {
	repository := &mockRepository{}
	groups := []*models.Group{
		{Name: "group name"},
	}
	groupsWithInstances := []GroupWithInstances{
		{
			Name: "group name",
			Instances: []*model.Instance{
				{
					Model:     gorm.Model{ID: 1},
					Name:      "instance name",
					GroupName: "group name",
				},
			},
		},
	}
	repository.
		On("FindByGroups", groups, true).
		Return(groupsWithInstances, nil)
	service := NewService(config.Config{}, repository, nil, nil, nil)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")

	handler.ListPresets(c)

	require.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusOK, groupsWithInstances)
	repository.AssertExpectations(t)
}

func TestHandler_ListPresets_RepositoryError(t *testing.T) {
	groups := []*models.Group{
		{Name: "group name"},
	}
	repository := &mockRepository{}
	repository.
		On("FindByGroups", groups, true).
		Return(nil, errors.New("some error"))
	service := NewService(config.Config{}, repository, nil, nil, nil)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")

	handler.ListPresets(c)

	require.Empty(t, w.Body.Bytes())
	require.Len(t, c.Errors, 1)
	require.ErrorContains(t, c.Errors[0].Err, "some error")
	repository.AssertExpectations(t)
}

func TestHandler_FindById(t *testing.T) {
	repository := &mockRepository{}
	instance := &model.Instance{
		Model:     gorm.Model{ID: 1},
		Name:      "instance name",
		GroupName: "group name",
	}
	repository.
		On("FindById", uint(1)).
		Return(instance, nil)
	service := NewService(config.Config{}, repository, nil, nil, nil)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")
	c.AddParam("id", "1")

	handler.FindById(c)

	require.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusOK, instance)
	repository.AssertExpectations(t)
}

func TestHandler_FindByIdDecrypted(t *testing.T) {
	repository := &mockRepository{}
	instance := &model.Instance{
		Model:     gorm.Model{ID: 1},
		UserID:    1,
		Name:      "instance name",
		GroupName: "group name",
	}
	repository.
		On("FindByIdDecrypted", uint(1)).
		Return(instance, nil)
	service := NewService(config.Config{}, repository, nil, nil, nil)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")
	c.AddParam("id", "1")

	handler.FindByIdDecrypted(c)

	require.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusOK, instance)
	repository.AssertExpectations(t)
}

func TestHandler_NameToId(t *testing.T) {
	repository := &mockRepository{}
	instance := &model.Instance{
		Model:     gorm.Model{ID: 1},
		Name:      "instance name",
		GroupName: "group name",
	}
	repository.
		On("FindByNameAndGroup", "instance name", "group name").
		Return(instance, nil)
	service := NewService(config.Config{}, repository, nil, nil, nil)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")
	c.AddParam("groupName", "group name")
	c.AddParam("instanceName", "instance name")

	handler.NameToId(c)

	require.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusOK, 1)
	repository.AssertExpectations(t)
}

func TestHandler_Delete(t *testing.T) {
	repository := &mockRepository{}
	instance := &model.Instance{
		Model:     gorm.Model{ID: 1},
		UserID:    1,
		Name:      "instance name",
		GroupName: "group name",
	}
	repository.
		On("FindById", uint(1)).
		Return(instance, nil)
	repository.
		On("FindByIdDecrypted", uint(1)).
		Return(instance, nil)
	repository.
		On("Unlink", &model.Instance{
			Model: gorm.Model{ID: 1},
		}).
		Return(nil)
	repository.
		On("Delete", uint(1)).
		Return(nil)
	service := NewService(config.Config{}, repository, nil, nil, nil)
	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group name")
	c.AddParam("id", "1")
	request, err := http.NewRequest(http.MethodDelete, "", nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "token")
	c.Request = request

	handler.Delete(c)

	require.Empty(t, c.Errors)
	require.Equal(t, http.StatusOK, w.Code)
	repository.AssertExpectations(t)
}

func newContext(w *httptest.ResponseRecorder, group string) *gin.Context {
	user := &models.User{
		ID: uint64(1),
		Groups: []*models.Group{
			{Name: group},
		},
	}
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)
	return c
}

func assertResponse[V any](t *testing.T, rec *httptest.ResponseRecorder, expectedCode int, expectedBody V) {
	require.Equal(t, expectedCode, rec.Code, "HTTP status code does not match")
	assertJSON(t, rec.Body, expectedBody)
}

func assertJSON[V any](t *testing.T, body *bytes.Buffer, expected V) {
	actualBody := new(V)
	err := json.Unmarshal(body.Bytes(), &actualBody)
	require.NoError(t, err)
	require.Equal(t, expected, *actualBody, "HTTP response body does not match")
}

type mockRepository struct{ mock.Mock }

func (m *mockRepository) Link(firstInstance, secondInstance *model.Instance) error {
	panic("implement me")
}

func (m *mockRepository) Unlink(instance *model.Instance) error {
	called := m.Called(instance)
	return called.Error(0)
}

func (m *mockRepository) Save(instance *model.Instance) error {
	called := m.Called(instance)
	return called.Error(0)
}

func (m *mockRepository) FindById(id uint) (*model.Instance, error) {
	called := m.Called(id)
	return called.Get(0).(*model.Instance), nil
}

func (m *mockRepository) FindByIdDecrypted(id uint) (*model.Instance, error) {
	called := m.Called(id)
	return called.Get(0).(*model.Instance), nil
}

func (m *mockRepository) FindByNameAndGroup(instance string, group string) (*model.Instance, error) {
	called := m.Called(instance, group)
	return called.Get(0).(*model.Instance), nil
}

func (m *mockRepository) FindByGroups(groups []*models.Group, presets bool) ([]GroupWithInstances, error) {
	called := m.Called(groups, presets)
	groupsWithInstances, ok := called.Get(0).([]GroupWithInstances)
	if ok {
		return groupsWithInstances, nil
	} else {
		return nil, called.Error(1)
	}
}

func (m *mockRepository) SaveDeployLog(instance *model.Instance, log string) error {
	called := m.Called(instance, log)
	return called.Error(0)
}

func (m *mockRepository) Delete(id uint) error {
	called := m.Called(id)
	return called.Error(0)
}

type mockUserClient struct{ mock.Mock }

func (m *mockUserClient) FindGroupByName(token string, name string) (*models.Group, error) {
	called := m.Called(token, name)
	return called.Get(0).(*models.Group), nil
}

type mockHelmfileService struct{ mock.Mock }

func (m *mockHelmfileService) sync(token string, instance *model.Instance, group *models.Group) (*exec.Cmd, error) {
	called := m.Called(token, instance, group)
	return called.Get(0).(*exec.Cmd), nil
}

func (m *mockHelmfileService) destroy(token string, instance *model.Instance, group *models.Group) (*exec.Cmd, error) {
	panic("implement me")
}
