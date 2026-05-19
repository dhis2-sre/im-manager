package handler

import (
	"context"
	"errors"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

const AdministratorGroupName = "administrators"

func CanWriteDeployment(user *model.User, deployment *model.Deployment) bool {
	return isAdministrator(user) ||
		isMemberOf(deployment.GroupName, user.AdminGroups) ||
		(user.ID == deployment.UserID && isMemberOf(deployment.GroupName, user.Groups))
}

func CanReadDeployment(user *model.User, deployment *model.Deployment) bool {
	return isAdministrator(user) || isMemberOf(deployment.GroupName, user.Groups)
}

func isMemberOf(groupName string, groups []model.Group) bool {
	for _, group := range groups {
		if groupName == group.Name {
			return true
		}
	}
	return false
}

func isAdministrator(user *model.User) bool {
	return isMemberOf(AdministratorGroupName, user.Groups)
}

// TODO: These are all related to databases and as such should probably be moved into the database package
func CanAccess(user *model.User, database *model.Database) bool {
	return IsAdministrator(user) ||
		IsGroupAdministrator(database.GroupName, user.AdminGroups) ||
		isMemberOf(database.GroupName, user.Groups)
}

func CanUnlock(user *model.User, database *model.Database) bool {
	return IsAdministrator(user) ||
		IsGroupAdministrator(database.GroupName, user.AdminGroups) ||
		hasLock(user, database)
}

func hasLock(user *model.User, database *model.Database) bool {
	return database.Lock != nil && user.ID == database.Lock.UserID
}

func IsAdministrator(user *model.User) bool {
	return isMemberOf(AdministratorGroupName, user.Groups)
}

func IsGroupAdministrator(groupName string, groups []model.Group) bool {
	return isMemberOf(groupName, groups)
}

func CanAccessGroup(user *model.User, groupName string) bool {
	return IsAdministrator(user) ||
		isMemberOf(groupName, user.Groups) ||
		IsGroupAdministrator(groupName, user.AdminGroups)
}

func CanAccessUser(caller *model.User, target *model.User) bool {
	return IsAdministrator(caller) || caller.ID == target.ID || sharesGroup(caller, target)
}

func sharesGroup(caller *model.User, target *model.User) bool {
	for _, g := range target.Groups {
		if isMemberOf(g.Name, caller.Groups) || isMemberOf(g.Name, caller.AdminGroups) {
			return true
		}
	}
	for _, g := range target.AdminGroups {
		if isMemberOf(g.Name, caller.Groups) || isMemberOf(g.Name, caller.AdminGroups) {
			return true
		}
	}
	return false
}

// GetUserFromContext returns the User value stored in ctx, if any otherwise it returns an error.
func GetUserFromContext(ctx context.Context) (*model.User, error) {
	user, ok := model.GetUserFromContext(ctx)
	if !ok {
		return nil, errors.New("user not found on context")
	}

	return user, nil
}
