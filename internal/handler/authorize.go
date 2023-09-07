package handler

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
)

const AdministratorGroupName = "administrators"

func CanReadInstance(user *model.User, instance *model.Instance) bool {
	return isAdministrator(user) || isMemberOf(instance.GroupName, user.Groups)
}

func CanWriteInstance(user *model.User, instance *model.Instance) bool {
	return isAdministrator(user) ||
		isMemberOf(instance.GroupName, user.AdminGroups) ||
		(user.ID == instance.UserID && isMemberOf(instance.GroupName, user.Groups))
}

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
