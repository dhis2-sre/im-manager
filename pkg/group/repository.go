package group

import (
	"errors"

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
