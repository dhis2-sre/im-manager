package instance

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/config"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
	panic("implement me")
}

func (m *mockRepository) Save(instance *model.Instance) error {
	panic("implement me")
}

func (m *mockRepository) FindById(id uint) (*model.Instance, error) {
	called := m.Called(id)
	return called.Get(0).(*model.Instance), nil
}

func (m *mockRepository) FindByIdDecrypted(id uint) (*model.Instance, error) {
	panic("implement me")
}

func (m *mockRepository) FindByNameAndGroup(instance string, group string) (*model.Instance, error) {
	panic("implement me")
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
	panic("implement me")
}

func (m *mockRepository) Delete(id uint) error {
	panic("implement me")
}
