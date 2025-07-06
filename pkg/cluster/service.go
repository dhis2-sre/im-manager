package cluster

import (
	"context"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

func NewService(clusterRepository *repository) Service {
	return Service{clusterRepository}
}

type groupService interface {
	Find(ctx context.Context, name string) (*model.Group, error)
}

type Service struct {
	clusterRepository *repository
}

func (s Service) Find(ctx context.Context, id uint) (model.Cluster, error) {
	return s.clusterRepository.find(ctx, id)
}

func (s Service) FindAll(ctx context.Context) ([]model.Cluster, error) {
	return s.clusterRepository.findAll(ctx)
}

func (s Service) Save(ctx context.Context, name, description string, kubernetesConfiguration []byte) (model.Cluster, error) {
	cluster := model.Cluster{
		Name:          name,
		Description:   description,
		Configuration: kubernetesConfiguration,
	}

	err := s.clusterRepository.save(ctx, &cluster)
	if err != nil {
		return model.Cluster{}, err
	}

	return cluster, nil
}

func (s Service) Update(ctx context.Context, id uint, name, description string, kubernetesConfiguration []byte) (model.Cluster, error) {
	cluster, err := s.clusterRepository.find(ctx, id)
	if err != nil {
		return model.Cluster{}, err
	}

	// Update fields only if provided
	if name != "" {
		cluster.Name = name
	}
	if description != "" {
		cluster.Description = description
	}
	if kubernetesConfiguration != nil {
		cluster.Configuration = kubernetesConfiguration
	}

	err = s.clusterRepository.update(ctx, cluster)
	if err != nil {
		return model.Cluster{}, err
	}

	return cluster, nil
}

func (s Service) Delete(ctx context.Context, id uint) error {
	cluster, err := s.clusterRepository.find(ctx, id)
	if err != nil {
		return err
	}

	return s.clusterRepository.delete(ctx, cluster)
}

func (s Service) FindOrCreate(ctx context.Context, name, description string) (model.Cluster, error) {
	return s.clusterRepository.findOrCreate(ctx, model.Cluster{
		Name:        name,
		Description: description,
	})
}
