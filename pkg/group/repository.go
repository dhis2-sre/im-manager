package group

import (
	"errors"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *repository {
	return &repository{db}
}

func (r repository) find(name string) (*model.Group, error) {
	var group *model.Group
	err := r.db.
		Preload("ClusterConfiguration").
		Where("name = ?", name).
		First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("group %q doesn't exist", name)
	}

	return group, err
}

const AdministratorGroupName = "administrators"

func (r repository) findAll(user *model.User) ([]model.Group, error) {
	groupsByName := make(map[string]struct{})
	for _, group := range user.Groups {
		groupsByName[group.Name] = struct{}{}
	}
	groupNames := maps.Keys(groupsByName)
	isAdmin := slices.Contains(groupNames, AdministratorGroupName)

	if isAdmin {
		var groups []model.Group
		err := r.db.Find(&groups).Error
		return groups, err
	}

	return findAllGroups(user), nil
}

func findAllGroups(user *model.User) []model.Group {
	var allGroups []model.Group
	allGroups = append(allGroups, user.Groups...)
	allGroups = append(allGroups, user.AdminGroups...)

	groupsByName := make(map[string]model.Group)
	for _, group := range allGroups {
		groupsByName[group.Name] = group
	}

	return maps.Values(groupsByName)
}

func (r repository) create(group *model.Group) error {
	return r.db.Create(&group).Error
}

func (r repository) findOrCreate(group *model.Group) (*model.Group, error) {
	var g *model.Group
	err := r.db.Where(model.Group{Name: group.Name}).Attrs(model.Group{Hostname: group.Hostname}).FirstOrCreate(&g).Error
	return g, err
}

func (r repository) addUser(group *model.Group, user *model.User) error {
	return r.db.Model(&group).Association("Users").Append([]*model.User{user})
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
