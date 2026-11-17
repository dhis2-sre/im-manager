package instance

import (
	"context"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
)

// SavePreset validates the preset's instances the way SaveDeployments does and creates it with them in one transaction.
func (s Service) SavePreset(ctx context.Context, preset *model.Deployment) error {
	stacksByName := map[string]*stack.Stack{}
	if err := s.validateNewDeployment(preset, stacksByName); err != nil {
		return err
	}

	return s.instanceRepository.SavePreset(ctx, preset, stacksByName)
}

func (s Service) FindPresetById(ctx context.Context, id uint) (*model.Deployment, error) {
	return s.instanceRepository.FindDeploymentById(ctx, id, true)
}

// PresetEdit is an edit to a preset, which unlike a deployment can also be renamed and moved to another group. A nil field keeps the preset's.
type PresetEdit struct {
	Edit
	Name  *string
	Group *model.Group
}

// EditPreset applies an edit to a preset. Nothing of a preset runs in the cluster, so a companion the edit switches off is deleted right away.
func (s Service) EditPreset(ctx context.Context, presetId uint, edit PresetEdit) (*model.Deployment, error) {
	preset, err := s.FindPresetById(ctx, presetId)
	if err != nil {
		return nil, err
	}

	decryptedPreset, err := s.decryptDeployment(preset)
	if err != nil {
		return nil, err
	}

	if edit.Name != nil {
		decryptedPreset.Name = *edit.Name
	}
	if edit.Group != nil {
		decryptedPreset.GroupName = edit.Group.Name
		decryptedPreset.Group = edit.Group
	}
	for _, deploymentInstance := range decryptedPreset.Instances {
		deploymentInstance.Name = decryptedPreset.Name
		deploymentInstance.GroupName = decryptedPreset.GroupName
		deploymentInstance.Group = decryptedPreset.Group
	}

	changes, err := s.saveEdit(ctx, decryptedPreset, edit.Edit)
	if err != nil {
		return nil, err
	}

	for _, deploymentInstance := range changes.Destroy {
		if err := s.DeleteDestroyedInstance(ctx, deploymentInstance); err != nil {
			return nil, err
		}
	}

	return changes.Deployment, nil
}

// DeploymentFromPreset is what the deployments started from a preset take in place of the preset's own values. A nil field keeps the preset's.
type DeploymentFromPreset struct {
	UserID      uint
	Group       *model.Group
	Description *string
	TTL         *uint
	Instances   map[string]InstanceEdit
}

// CreateDeploymentsFromPreset saves one deployment per name in one transaction, each holding the preset's instances with the request applied and in the given deploy status. The preset's sensitive values carry over, which a client only ever sees masked.
func (s Service) CreateDeploymentsFromPreset(ctx context.Context, presetId uint, names []string, request DeploymentFromPreset, status model.DeployStatus) ([]*model.Deployment, error) {
	preset, err := s.FindPresetById(ctx, presetId)
	if err != nil {
		return nil, err
	}

	decryptedPreset, err := s.decryptDeployment(preset)
	if err != nil {
		return nil, err
	}

	deployments, err := s.newDeploymentsFromPreset(decryptedPreset, names, request, status)
	if err != nil {
		return nil, err
	}

	if err := s.SaveDeployments(ctx, request.Group.Name, deployments); err != nil {
		return nil, err
	}

	return deployments, nil
}

func (s Service) newDeploymentsFromPreset(preset *model.Deployment, names []string, request DeploymentFromPreset, status model.DeployStatus) ([]*model.Deployment, error) {
	deployments := make([]*model.Deployment, len(names))
	for i, name := range names {
		deployment, err := s.newDeploymentFromPreset(preset, name, request)
		if err != nil {
			return nil, err
		}
		for _, deploymentInstance := range deployment.Instances {
			deploymentInstance.DeployStatus = status
		}
		deployments[i] = deployment
	}
	return deployments, nil
}

// newDeploymentFromPreset builds the deployment a preset starts. Consumed parameters are resolved again for the new name and group, and parameters the stack no longer declares are dropped.
func (s Service) newDeploymentFromPreset(preset *model.Deployment, name string, request DeploymentFromPreset) (*model.Deployment, error) {
	deployment := &model.Deployment{
		UserID:      request.UserID,
		Name:        name,
		GroupName:   request.Group.Name,
		Group:       request.Group,
		Description: preset.Description,
		TTL:         preset.TTL,
	}
	if request.Description != nil {
		deployment.Description = *request.Description
	}
	if request.TTL != nil {
		deployment.TTL = *request.TTL
	}

	for _, presetInstance := range preset.Instances {
		instanceStack, err := s.stackService.Find(presetInstance.StackName)
		if err != nil {
			return nil, err
		}

		parameters := make(model.DeploymentInstanceParameters, len(presetInstance.Parameters))
		for name, parameter := range presetInstance.Parameters {
			if stackParameter, ok := instanceStack.Parameters[name]; !ok || stackParameter.Consumed {
				continue
			}
			parameters[name] = model.DeploymentInstanceParameter{ParameterName: name, Value: parameter.Value}
		}

		deployment.Instances = append(deployment.Instances, &model.DeploymentInstance{
			Name:       deployment.Name,
			Group:      deployment.Group,
			GroupName:  deployment.GroupName,
			StackName:  presetInstance.StackName,
			Public:     presetInstance.Public,
			Parameters: parameters,
		})
	}

	added, switchedOff, err := s.applyEdits(deployment, request.Instances)
	if err != nil {
		return nil, err
	}
	deployment.Instances = instancesAfterEdits(deployment.Instances, added, switchedOff)

	return deployment, nil
}
