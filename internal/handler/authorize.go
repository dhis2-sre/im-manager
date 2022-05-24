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
