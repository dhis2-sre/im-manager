package group

import (
	"context"

	"github.com/dhis2-sre/im-manager/pkg/instance"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

func NewService(groupRepository *repository, userService userService) *Service {
	return &Service{
		groupRepository: groupRepository,
		userService:     userService,
	}
}

type userService interface {
	FindById(ctx context.Context, id uint) (*model.User, error)
}

type Service struct {
	groupRepository *repository
	userService     userService
}

func (s *Service) Find(ctx context.Context, name string) (*model.Group, error) {
	return s.groupRepository.find(ctx, name)
}

func (s *Service) FindWithDetails(ctx context.Context, name string) (*model.Group, error) {
	return s.groupRepository.findWithDetails(ctx, name)
}

func (s *Service) Create(ctx context.Context, name, namespace, description, hostname string, deployable bool) (*model.Group, error) {
	group := &model.Group{
		Name:        name,
		Namespace:   namespace,
		Description: description,
		Hostname:    hostname,
		Deployable:  deployable,
	}

	err := s.groupRepository.create(ctx, group)
	if err != nil {
		return nil, err
	}

	return group, err
}

func (s *Service) FindOrCreate(ctx context.Context, name, namespace, hostname string, deployable bool) (*model.Group, error) {
	group := &model.Group{
		Name:       name,
		Namespace:  namespace,
		Hostname:   hostname,
		Deployable: deployable,
	}

	g, err := s.groupRepository.findOrCreate(ctx, group)
	if err != nil {
		return nil, err
	}

	return g, err
}

func (s *Service) AddUser(ctx context.Context, groupName string, userId uint) error {
	group, err := s.Find(ctx, groupName)
	if err != nil {
		return err
	}

	u, err := s.userService.FindById(ctx, userId)
	if err != nil {
		return err
	}

	return s.groupRepository.addUser(ctx, group, u)
}

func (s *Service) RemoveUser(ctx context.Context, groupName string, userId uint) error {
	group, err := s.Find(ctx, groupName)
	if err != nil {
		return err
	}

	u, err := s.userService.FindById(ctx, userId)
	if err != nil {
		return err
	}

	return s.groupRepository.removeUser(ctx, group, u)
}

func (s *Service) AddClusterConfiguration(ctx context.Context, clusterConfiguration *model.ClusterConfiguration) error {
	return s.groupRepository.addClusterConfiguration(ctx, clusterConfiguration)
}

func (s *Service) GetClusterConfiguration(ctx context.Context, groupName string) (*model.ClusterConfiguration, error) {
	return s.groupRepository.getClusterConfiguration(ctx, groupName)
}

func (s *Service) FindAll(ctx context.Context, user *model.User, deployable bool) ([]model.Group, error) {
	return s.groupRepository.findAll(ctx, user, deployable)
}

func (s *Service) FindByGroupNames(ctx context.Context, groupNames []string) ([]model.Group, error) {
	return s.groupRepository.findByGroupNames(ctx, groupNames)
}

func (s *Service) FindResources(ctx context.Context, name string) (instance.ClusterResources, error) {
	group, err := s.groupRepository.find(ctx, name)
	if err != nil {
		return instance.ClusterResources{}, err
	}

	resources, err := instance.FindResources(group.ClusterConfiguration)
	if err != nil {
		return instance.ClusterResources{}, err
	}

	resources.Autoscaled = group.Autoscaled

	return resources, nil
}
