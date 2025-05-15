package user

import (
	"context"
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

type groupService interface {
	FindOrCreate(ctx context.Context, name string, namespace string, hostname string, deployable bool) (*model.Group, error)
	AddUser(ctx context.Context, group string, userId uint) error
}

func CreateUser(ctx context.Context, email, password string, userService *Service, groupService groupService, groupName, namespace, userType string) error {
	u, err := userService.FindOrCreate(ctx, email, password)
	if err != nil {
		return fmt.Errorf("error creating %s user: %v", userType, err)
	}

	u.Validated = true

	err = userService.Save(ctx, u)
	if err != nil {
		return fmt.Errorf("error saving %s user: %v", userType, err)
	}

	g, err := groupService.FindOrCreate(ctx, groupName, namespace, "", false)
	if err != nil {
		return fmt.Errorf("error creating %s group: %v", groupName, err)
	}

	err = groupService.AddUser(ctx, g.Name, u.ID)
	if err != nil {
		return fmt.Errorf("error adding %s user to %s group: %v", userType, groupName, err)
	}

	return nil
}
