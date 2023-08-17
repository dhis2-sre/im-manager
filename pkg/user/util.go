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

func CreateAdminUser(email, password string, userService userServiceUtil, groupService groupService) error {
	u, err := userService.FindOrCreate(email, password)
	if err != nil {
		return fmt.Errorf("error creating admin user: %v", err)
	}

	u.Validated = true

	err = userService.Save(u)
	if err != nil {
		return fmt.Errorf("error saving admin user: %v", err)
	}

	g, err := groupService.FindOrCreate(model.AdministratorGroupName, "", false)
	if err != nil {
		return fmt.Errorf("error creating admin group: %v", err)
	}

	err = groupService.AddUser(g.Name, u.ID)
	if err != nil {
		return fmt.Errorf("error adding admin user to admin group: %v", err)
	}

	return nil
}
