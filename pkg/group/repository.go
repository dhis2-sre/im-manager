package group

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB) *repository {
	return &repository{
		db: db,
	}
}

func (r repository) find(ctx context.Context, name string) (*model.Group, error) {
	var group *model.Group
	err := r.db.
		WithContext(ctx).
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

func (r repository) findWithDetails(ctx context.Context, name string) (*model.Group, error) {
	var group *model.Group
	err := r.db.
		WithContext(ctx).
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

const AdministratorGroupName = "administrators"

func (r repository) findAll(ctx context.Context, user *model.User, deployable bool) ([]model.Group, error) {
	groupsByName := make(map[string]struct{})
	for _, group := range user.Groups {
		groupsByName[group.Name] = struct{}{}
	}
	groupNames := maps.Keys(groupsByName)
	isAdmin := slices.Contains(groupNames, AdministratorGroupName)

	if isAdmin {
		var groups []model.Group
		if deployable {
			err := r.db.
				WithContext(ctx).
				Where("deployable = true").
				Find(&groups).Error
			return groups, err
		}
		err := r.db.WithContext(ctx).Find(&groups).Error
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

func (r repository) create(ctx context.Context, group *model.Group) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	err := r.db.WithContext(ctx).Create(&group).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		// TODO how to check if name or hostname is duplicated?
		return errdef.NewDuplicated("group name/hostname already exists: %s", err)
	}

	return err
}

func (r repository) findOrCreate(ctx context.Context, group *model.Group) (*model.Group, error) {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	var g *model.Group
	err := r.db.
		WithContext(ctx).
		Where(model.Group{Name: group.Name}).
		Attrs(model.Group{Hostname: group.Hostname, Deployable: group.Deployable}).
		FirstOrCreate(&g).Error
	return g, err
}

func (r repository) addUser(ctx context.Context, group *model.Group, user *model.User) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	return r.db.WithContext(ctx).Model(&group).Association("Users").Append([]*model.User{user})
}

func (r repository) removeUser(ctx context.Context, group *model.Group, user *model.User) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	return r.db.WithContext(ctx).Model(&group).Association("Users").Delete([]*model.User{user})
}

func (r repository) addClusterConfiguration(ctx context.Context, configuration *model.ClusterConfiguration) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	return r.db.WithContext(ctx).Create(&configuration).Error
}

func (r repository) getClusterConfiguration(ctx context.Context, groupName string) (*model.ClusterConfiguration, error) {
	var configuration *model.ClusterConfiguration
	err := r.db.
		WithContext(ctx).
		Where("group_name = ?", groupName).
		First(&configuration).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("group %q doesn't exist", groupName)
	}
	return configuration, err
}

func (r repository) findByGroupNames(ctx context.Context, groupNames []string) ([]model.Group, error) {
	var databases []model.Group
	err := r.db.
		WithContext(ctx).
		Where("name IN ?", groupNames).
		Order("updated_at desc").
		Find(&databases).Error
	return databases, err
}
