package handler

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
)

const AdministratorGroupName = "administrators"

func CanReadInstance(user *models.User, instance *model.Instance) bool {
	return isAdministrator(user) || isMemberOf(instance.GroupName, user.Groups)
}

func CanWriteInstance(user *models.User, instance *model.Instance) bool {
	return isAdministrator(user) ||
		isMemberOf(instance.GroupName, user.AdminGroups) ||
		(isOwner(user, instance) && isMemberOf(instance.GroupName, user.Groups))
}

func isOwner(user *models.User, instance *model.Instance) bool {
	return uint(user.ID) == instance.UserID
}

func isMemberOf(groupName string, groups []*models.Group) bool {
	for _, group := range groups {
		if groupName == group.Name {
			return true
		}
	}
	return false
}

func isAdministrator(user *models.User) bool {
	return isMemberOf(AdministratorGroupName, user.Groups)
}

// TODO: These are all related to databases and as such should probably be moved into the database package
func CanAccess(user *models.User, database *model.Database) bool {
	return IsAdministrator(user) ||
		IsGroupAdministrator(database.GroupName, user.AdminGroups) ||
		isMemberOf(database.GroupName, user.Groups)
}

func CanUnlock(user *models.User, database *model.Database) bool {
	return IsAdministrator(user) ||
		IsGroupAdministrator(database.GroupName, user.AdminGroups) ||
		hasLock(user, database)
}

func hasLock(user *models.User, database *model.Database) bool {
	return database.Lock != nil && uint(user.ID) == database.Lock.UserID
}

func IsAdministrator(user *models.User) bool {
	return isMemberOf(AdministratorGroupName, user.Groups)
}

func IsGroupAdministrator(groupName string, groups []*models.Group) bool {
	return isMemberOf(groupName, groups)
}
