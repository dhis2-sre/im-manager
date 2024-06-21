package event

import (
	"log/slog"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/stretchr/testify/assert"
)

func TestEventPostFilter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	w := httptest.ResponseRecorder{}
	c := gin.CreateTestContextOnly(&w, &gin.Engine{})

	var user1 uint = 1
	user1GroupsMap := map[string]struct{}{
		"group1": {},
		"group3": {},
		"group4": {},
		"group7": {},
	}
	var nonMatchingUserID uint = 17
	nonMatchingGroup := "group17"
	tests := map[string]struct {
		userID        uint
		userGroupsMap map[string]struct{}
		message       amqp.Message
		want          bool
	}{
		"MessageForAnyone": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message:       amqp.Message{},
			want:          true,
		},
		"MessageForGroupMatchingTheUsersGroup": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": "group3"},
			},
			want: true,
		},
		"MessageForUserMatchingTheUser": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"owner": strconv.Itoa(int(user1))},
			},
			want: true,
		},
		"MessageForGroupAndUserMatchingTheUser": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": "group4", "owner": strconv.Itoa(int(user1))},
			},
			want: true,
		},
		"MessageForGroupNotMatchingTheUsersGroup": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": "group10"},
			},
			want: false,
		},
		"MessageForGroupAndUserNotMatchingTheUsersID": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": "group7", "owner": strconv.Itoa(int(nonMatchingUserID))},
			},
			want: false,
		},
		"MessageForGroupAndUserNotMatchingTheUsersGroup": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": nonMatchingGroup, "owner": strconv.Itoa(int(user1))},
			},
			want: false,
		},
		"MessageForGroupAndUserNotMatchingTheUsersIDAndGroup": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": nonMatchingGroup, "owner": strconv.Itoa(int(nonMatchingUserID))},
			},
			want: false,
		},
		"MessageForGroupAndUserWithNonStringOwner": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": nonMatchingGroup, "owner": user1},
			},
			want: false,
		},
		"MessageForGroupAndUserWithNonUintOwner": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": nonMatchingGroup, "owner": "wrong"},
			},
			want: false,
		},
		"MessageForGroupWithNonStringGroup": {
			userID:        user1,
			userGroupsMap: user1GroupsMap,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": 1},
			},
			want: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := postFilter(c, logger, test.userID, test.userGroupsMap)(&test.message)

			assert.Equal(t, test.want, got)
		})
	}
}
