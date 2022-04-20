package handler

import (
	"sort"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
)

const AdministratorGroupName = "administrators"

func CanReadInstance(user *models.User, instance *model.Instance) bool {
	return isAdministrator(user) || isMemberOf(instance.GroupID, user.Groups)
}

func CanWriteInstance(user *models.User, instance *model.Instance) bool {
	return isAdministrator(user) ||
		isMemberOf(instance.GroupID, user.AdminGroups) ||
		(isOwner(user, instance) && isMemberOf(instance.GroupID, user.Groups))
}

func isOwner(user *models.User, instance *model.Instance) bool {
	return uint(user.ID) == instance.UserID
}

func isMemberOf(groupId uint, groups []*models.Group) bool {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID <= groups[j].ID
	})

	index := sort.Search(len(groups), func(i int) bool {
		return uint(groups[i].ID) >= groupId
	})

	return index < len(groups) && uint(groups[index].ID) == groupId
}

func isAdministrator(user *models.User) bool {
	return contains(AdministratorGroupName, user.Groups)
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
