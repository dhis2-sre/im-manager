package event

import (
	"log/slog"
	"os"
	"strconv"
	"testing"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/stretchr/testify/assert"
)

func TestEventPostFilter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var user1 uint = 1
	user1Groups := []string{
		"group1",
		"group3",
		"group4",
		"group7",
	}
	var nonMatchingUserID uint = 17
	nonMatchingGroup := "group17"
	tests := map[string]struct {
		userID     uint
		userGroups []string
		message    amqp.Message
		want       bool
	}{
		"MessageForAnyone": {
			userID:     user1,
			userGroups: user1Groups,
			message:    amqp.Message{},
			want:       true,
		},
		"MessageForGroupMatchingTheUsersGroup": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": user1Groups[1]},
			},
			want: true,
		},
		"MessageForUserMatchingTheUser": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"owner": strconv.Itoa(int(user1))},
			},
			want: true,
		},
		"MessageForGroupAndUserMatchingTheUser": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": user1Groups[2], "owner": strconv.Itoa(int(user1))},
			},
			want: true,
		},
		"MessageForGroupNotMatchingTheUsersGroup": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": "group10"},
			},
			want: false,
		},
		"MessageForGroupAndUserNotMatchingTheUsersID": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": user1Groups[3], "owner": strconv.Itoa(int(nonMatchingUserID))},
			},
			want: false,
		},
		"MessageForGroupAndUserNotMatchingTheUsersGroup": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": nonMatchingGroup, "owner": strconv.Itoa(int(user1))},
			},
			want: false,
		},
		"MessageForGroupAndUserNotMatchingTheUsersIDAndGroup": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": nonMatchingGroup, "owner": strconv.Itoa(int(nonMatchingUserID))},
			},
			want: false,
		},
		"MessageForGroupAndUserWithNonStringOwner": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": nonMatchingGroup, "owner": user1},
			},
			want: false,
		},
		"MessageForGroupAndUserWithNonUintOwner": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": nonMatchingGroup, "owner": "wrong"},
			},
			want: false,
		},
		"MessageForGroupWithNonStringGroup": {
			userID:     user1,
			userGroups: user1Groups,
			message: amqp.Message{
				ApplicationProperties: map[string]any{"group": 1},
			},
			want: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := postFilter(logger, test.userID, test.userGroups)(&test.message)

			assert.Equal(t, test.want, got)
		})
	}
}
