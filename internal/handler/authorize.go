package handler

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"sort"
)

const AdministratorGroupName = "administrators"

func CanReadInstance(user *models.User, instance *model.Instance) bool {
	return isAdministrator(user) || isMemberOfById(user, instance.GroupID)
}

func CanWriteInstance(user *models.User, instance *model.Instance) bool {
	return isAdministrator(user) || (uint(user.ID) == instance.UserID && isMemberOfById(user, instance.GroupID))
}

func isMemberOfById(user *models.User, groupId uint) bool {
	groups := user.Groups

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID <= groups[j].ID
	})

	index := sort.Search(len(groups), func(i int) bool {
		return uint(groups[i].ID) >= groupId
	})

	return index < len(groups) && uint(groups[index].ID) == groupId
}

func isMemberOf(user *models.User, groupName string) bool {
	return contains(groupName, user.Groups)
}

func IsAdminOf(user *models.User, groupName string) bool {
	return contains(groupName, user.AdminGroups)
}

func contains(groupName string, groups []*models.Group) bool {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name <= groups[j].Name
	})

	index := sort.Search(len(groups), func(i int) bool {
		return groups[i].Name >= groupName
	})

	return index < len(groups) && groups[index].Name == groupName
}

func isAdministrator(user *models.User) bool {
	return isMemberOf(user, AdministratorGroupName)
}
