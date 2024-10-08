package group

import (
	"context"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(groupRepository groupRepository, userService userService) *service {
	return &service{
		groupRepository,
		userService,
	}
}

type groupRepository interface {
	create(group *model.Group) error
	addUser(group *model.Group, user *model.User) error
	removeUser(group *model.Group, user *model.User) error
	addClusterConfiguration(configuration *model.ClusterConfiguration) error
	getClusterConfiguration(groupName string) (*model.ClusterConfiguration, error)
	find(name string) (*model.Group, error)
	findWithDetails(name string) (*model.Group, error)
	findOrCreate(group *model.Group) (*model.Group, error)
	findAll(user *model.User, deployable bool) ([]model.Group, error)
	findByGroupNames(groupNames []string) ([]model.Group, error)
}

type userService interface {
	FindById(ctx context.Context, id uint) (*model.User, error)
}

type service struct {
	groupRepository groupRepository
	userService     userService
}

func (s *service) Find(name string) (*model.Group, error) {
	return s.groupRepository.find(name)
}

func (s *service) FindWithDetails(name string) (*model.Group, error) {
	return s.groupRepository.findWithDetails(name)
}

func (s *service) Create(name, description, hostname string, deployable bool) (*model.Group, error) {
	group := &model.Group{
		Name:        name,
		Description: description,
		Hostname:    hostname,
		Deployable:  deployable,
	}

	err := s.groupRepository.create(group)
	if err != nil {
		return nil, err
	}

	return group, err
}

func (s *service) FindOrCreate(name string, hostname string, deployable bool) (*model.Group, error) {
	group := &model.Group{
		Name:       name,
		Hostname:   hostname,
		Deployable: deployable,
	}

	g, err := s.groupRepository.findOrCreate(group)
	if err != nil {
		return nil, err
	}

	return g, err
}

func (s *service) AddUser(ctx context.Context, groupName string, userId uint) error {
	group, err := s.Find(groupName)
	if err != nil {
		return err
	}

	u, err := s.userService.FindById(ctx, userId)
	if err != nil {
		return err
	}

	return s.groupRepository.addUser(group, u)
}

func (s *service) RemoveUser(c context.Context, groupName string, userId uint) error {
	group, err := s.Find(groupName)
	if err != nil {
		return err
	}

	u, err := s.userService.FindById(c, userId)
	if err != nil {
		return err
	}

	return s.groupRepository.removeUser(group, u)
}

func (s *service) AddClusterConfiguration(clusterConfiguration *model.ClusterConfiguration) error {
	return s.groupRepository.addClusterConfiguration(clusterConfiguration)
}

func (s *service) GetClusterConfiguration(groupName string) (*model.ClusterConfiguration, error) {
	return s.groupRepository.getClusterConfiguration(groupName)
}

func (s *service) FindAll(user *model.User, deployable bool) ([]model.Group, error) {
	return s.groupRepository.findAll(user, deployable)
}

func (s *service) FindByGroupNames(groupNames []string) ([]model.Group, error) {
	return s.groupRepository.findByGroupNames(groupNames)
}
