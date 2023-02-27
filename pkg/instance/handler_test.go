package instance

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHandler_List_ServiceError(t *testing.T) {
	groups := []*models.Group{
		{
			Name:     "name",
			Hostname: "hostname",
		},
	}

	service := &mockInstanceService{}
	errorMessage := "some error"
	service.
		On("FindInstances", groups, false).
		Return(nil, errors.New(errorMessage))

	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", &models.User{Groups: groups})

	handler.ListInstances(c)

	assert.Empty(t, w.Body.Bytes())
	assert.NotEmpty(t, c.Errors)

	service.AssertExpectations(t)
}

func TestHandler_List(t *testing.T) {
	groups := []*models.Group{
		{
			Name:     "name",
			Hostname: "hostname",
		},
	}

	instances := []*model.Instance{
		{
			Model:     gorm.Model{ID: 1},
			Name:      "some name",
			GroupName: groups[0].Name,
		},
	}

	service := &mockInstanceService{}
	service.
		On("FindInstances", groups, false).
		Return(instances, nil)

	handler := NewHandler(nil, service, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", &models.User{Groups: groups})

	handler.ListInstances(c)

	assert.Empty(t, c.Errors)
	assert.Equal(t, http.StatusOK, w.Code)
	var body []GroupWithInstances
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	//	assert.Equal(t, groupsWithInstances(groups, instances), body)

	service.AssertExpectations(t)
}

type mockInstanceService struct{ mock.Mock }

func (m *mockInstanceService) ConsumeParameters(source, destination *model.Instance) error {
	panic("implement me")
}

func (m *mockInstanceService) Pause(token string, instance *model.Instance) error {
	panic("implement me")
}

func (m *mockInstanceService) Restart(token string, instance *model.Instance, typeSelector string) error {
	panic("implement me")
}

func (m *mockInstanceService) Save(instance *model.Instance) (*model.Instance, error) {
	panic("implement me")
}

func (m *mockInstanceService) Deploy(token string, instance *model.Instance) error {
	panic("implement me")
}

func (m *mockInstanceService) FindById(id uint) (*model.Instance, error) {
	panic("implement me")
}

func (m *mockInstanceService) FindByIdDecrypted(id uint) (*model.Instance, error) {
	panic("implement me")
}

func (m *mockInstanceService) FindByNameAndGroup(instance string, group string) (*model.Instance, error) {
	panic("implement me")
}

func (m *mockInstanceService) Delete(token string, id uint) error {
	panic("implement me")
}

func (m *mockInstanceService) Logs(instance *model.Instance, group *models.Group, typeSelector string) (io.ReadCloser, error) {
	panic("implement me")
}

func (m *mockInstanceService) FindInstances(user *models.User, presets bool) ([]GroupWithInstances, error) {
	called := m.Called(user, presets)
	instances, ok := called.Get(0).([]GroupWithInstances)
	if ok {
		return instances, nil
	} else {
		return nil, called.Error(1)
	}
}

func (m *mockInstanceService) Link(source, destination *model.Instance) error {
	panic("implement me")
}
