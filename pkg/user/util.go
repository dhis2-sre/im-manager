package user

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

type groupService interface {
	FindOrCreate(group, hostname string, deployable bool) (*model.Group, error)
	AddUser(group string, userId uint) error
}

type userServiceUtil interface {
	FindOrCreate(email, password string) (*model.User, error)
	Save(user *model.User) error
}

func CreateUser(email, password string, userService userServiceUtil, groupService groupService, groupName, userType string) error {
	u, err := userService.FindOrCreate(email, password)
	if err != nil {
		return fmt.Errorf("error creating %s user: %v", userType, err)
	}

	u.Validated = true

	err = userService.Save(u)
	if err != nil {
		return fmt.Errorf("error saving %s user: %v", userType, err)
	}

	g, err := groupService.FindOrCreate(groupName, "", false)
	if err != nil {
		return fmt.Errorf("error creating %s group: %v", groupName, err)
	}

	err = groupService.AddUser(g.Name, u.ID)
	if err != nil {
		return fmt.Errorf("error adding %s user to %s group: %v", userType, groupName, err)
	}

	return nil
}
