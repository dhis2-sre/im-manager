package notification_test

import (
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/notification"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestListSubjectExists(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)

	member := model.Group{Name: "member", Hostname: "member"}
	administered := model.Group{Name: "administered", Hostname: "administered"}
	other := model.Group{Name: "other", Hostname: "other"}
	administrators := model.Group{Name: model.AdministratorGroupName, Hostname: model.AdministratorGroupName}
	require.NoError(t, db.Create([]*model.Group{&member, &administered, &other, &administrators}).Error)

	owner := model.User{Email: "owner@dhis2.org"}
	require.NoError(t, db.Create(&owner).Error)

	createDeployment := func(name, groupName string, preset bool) uint {
		d := model.Deployment{Name: name, GroupName: groupName, UserID: owner.ID, Preset: preset}
		require.NoError(t, db.Create(&d).Error)
		return d.ID
	}
	createDatabase := func(name, groupName string) uint {
		d := model.Database{Name: name, GroupName: groupName, Slug: name, Type: "database", UserID: owner.ID}
		require.NoError(t, db.Create(&d).Error)
		return d.ID
	}

	deploymentInMember := createDeployment("in-member", member.Name, false)
	deploymentInAdministered := createDeployment("in-administered", administered.Name, false)
	deploymentInOther := createDeployment("in-other", other.Name, false)
	preset := createDeployment("preset", member.Name, true)
	deletedDeployment := createDeployment("deleted", member.Name, false)
	require.NoError(t, db.Delete(&model.Deployment{}, deletedDeployment).Error)

	databaseInMember := createDatabase("in-member.sql.gz", member.Name)
	databaseInAdministered := createDatabase("in-administered.sql.gz", administered.Name)
	databaseInOther := createDatabase("in-other.sql.gz", other.Name)
	deletedDatabase := createDatabase("deleted.sql.gz", member.Name)
	require.NoError(t, db.Delete(&model.Database{}, deletedDatabase).Error)

	notifications := map[string]model.Notification{}
	notify := func(name, kind, data string) {
		n := model.Notification{UserID: owner.ID, GroupName: member.Name, Kind: kind, Data: data}
		require.NoError(t, db.Create(&n).Error)
		notifications[name] = n
	}
	deployment := func(id uint) string {
		return fmt.Sprintf(`{"status":"success","deploymentId":%d,"deploymentName":"name"}`, id)
	}
	database := func(id uint) string {
		return fmt.Sprintf(`{"status":"success","databaseId":%d,"databaseName":"name"}`, id)
	}
	notify("deploymentInMember", "deployment", deployment(deploymentInMember))
	notify("deploymentInAdministered", "deployment", deployment(deploymentInAdministered))
	notify("deploymentInOther", "deployment", deployment(deploymentInOther))
	notify("preset", "deployment", deployment(preset))
	notify("deletedDeployment", "deployment", deployment(deletedDeployment))
	notify("databaseInMember", "database-save", database(databaseInMember))
	notify("filestoreInMember", "filestore-backup", database(databaseInMember))
	notify("databaseInAdministered", "database-save", database(databaseInAdministered))
	notify("databaseInOther", "filestore-backup", database(databaseInOther))
	notify("deletedDatabase", "database-save", database(deletedDatabase))
	notify("unknownKind", "something-else", database(databaseInMember))
	notify("malformedData", "deployment", "not json")
	notify("missingId", "database-save", `{"status":"started"}`)

	var queries atomic.Int64
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("count_queries", func(*gorm.DB) {
		queries.Add(1)
	}))

	list := func(t *testing.T, user *model.User) map[string]*bool {
		t.Helper()

		repository := notification.NewRepository(db)
		h := notification.NewHandler(slog.New(slog.NewTextHandler(os.Stdout, nil)), notification.NewService(repository))
		client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
			authenticator := func(c *gin.Context) {
				c.Request = c.Request.WithContext(model.NewContextWithUser(c.Request.Context(), user))
			}
			notification.Routes(engine, authenticator, h)
		})

		before := queries.Load()
		var listed []model.Notification
		client.GetJSON(t, "/notifications", &listed)
		assert.Equal(t, int64(3), queries.Load()-before, "the notifications, then one query for deployments and one for databases")

		ids := map[uint]*bool{}
		for _, n := range listed {
			ids[n.ID] = n.SubjectExists
		}
		exists := map[string]*bool{}
		for name, n := range notifications {
			subjectExists, ok := ids[n.ID]
			require.True(t, ok, "notification %q is listed", name)
			exists[name] = subjectExists
		}
		return exists
	}

	yes, no := new(true), new(false)

	t.Run("MemberAndGroupAdministrator", func(t *testing.T) {
		user := &model.User{ID: owner.ID, Groups: []model.Group{member}, AdminGroups: []model.Group{administered}}

		exists := list(t, user)

		assert.Equal(t, map[string]*bool{
			"deploymentInMember":       yes,
			"deploymentInAdministered": no,
			"deploymentInOther":        no,
			"preset":                   no,
			"deletedDeployment":        no,
			"databaseInMember":         yes,
			"filestoreInMember":        yes,
			"databaseInAdministered":   yes,
			"databaseInOther":          no,
			"deletedDatabase":          no,
			"unknownKind":              nil,
			"malformedData":            nil,
			"missingId":                nil,
		}, exists)
	})

	t.Run("Administrator", func(t *testing.T) {
		user := &model.User{ID: owner.ID, Groups: []model.Group{administrators}}

		exists := list(t, user)

		assert.Equal(t, map[string]*bool{
			"deploymentInMember":       yes,
			"deploymentInAdministered": yes,
			"deploymentInOther":        yes,
			"preset":                   no,
			"deletedDeployment":        no,
			"databaseInMember":         yes,
			"filestoreInMember":        yes,
			"databaseInAdministered":   yes,
			"databaseInOther":          yes,
			"deletedDatabase":          no,
			"unknownKind":              nil,
			"malformedData":            nil,
			"missingId":                nil,
		}, exists)
	})
}
