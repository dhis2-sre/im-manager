package group

import (
	"errors"
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"golang.org/x/exp/maps"
	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB) *repository {
	return &repository{db}
}

func (r repository) find(name string) (*model.Group, error) {
	var group *model.Group
	err := r.db.
		Joins("ClusterConfiguration").
		Where("name = ?", name).
		First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("group %q doesn't exist", name)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find group: %v", err)
	}

	return group, nil
}

func (r repository) findWithDetails(name string) (*model.Group, error) {
	var group *model.Group
	err := r.db.
		Where("name = ?", name).
		Joins("ClusterConfiguration").
		Preload("Users").
		Preload("AdminUsers").
		First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("group %q doesn't exist", name)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find group with details: %v", err)
	}

	return group, nil
}

func (r repository) findAll(user *model.User, deployable bool) ([]model.Group, error) {
	if user.IsAdministrator() {
		var groups []model.Group
		if deployable {
			err := r.db.
				Where("deployable = true").
				Find(&groups).Error
			return groups, err
		}
		err := r.db.Find(&groups).Error
		return groups, err
	}

	return findAllFromUser(user, deployable), nil
}

func findAllFromUser(user *model.User, deployable bool) []model.Group {
	var allGroups []model.Group
	allGroups = append(allGroups, user.Groups...)
	allGroups = append(allGroups, user.AdminGroups...)

	if deployable {
		index := 0
		for _, group := range allGroups {
			if group.Deployable {
				allGroups[index] = group
				index++
			}
		}
		allGroups = allGroups[:index]
	}

	groupsByName := make(map[string]model.Group)
	for _, group := range allGroups {
		groupsByName[group.Name] = group
	}

	return maps.Values(groupsByName)
}

func (r repository) create(group *model.Group) error {
	err := r.db.Create(&group).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		// TODO how to check if name or hostname is duplicated?
		return errdef.NewDuplicated("group name/hostname already exists: %s", err)
	}

	return err
}

func (r repository) findOrCreate(group *model.Group) (*model.Group, error) {
	var g *model.Group
	err := r.db.
		Where(model.Group{Name: group.Name}).
		Attrs(model.Group{Hostname: group.Hostname, Deployable: group.Deployable}).
		FirstOrCreate(&g).Error
	return g, err
}

func (r repository) addUser(group *model.Group, user *model.User) error {
	return r.db.Model(&group).Association("Users").Append([]*model.User{user})
}

func (r repository) removeUser(group *model.Group, user *model.User) error {
	return r.db.Model(&group).Association("Users").Delete([]*model.User{user})
}

func (r repository) addClusterConfiguration(configuration *model.ClusterConfiguration) error {
	return r.db.Create(&configuration).Error
}

func (r repository) getClusterConfiguration(groupName string) (*model.ClusterConfiguration, error) {
	var configuration *model.ClusterConfiguration
	err := r.db.
		Where("group_name = ?", groupName).
		First(&configuration).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("group %q doesn't exist", groupName)
	}
	return configuration, err
}
