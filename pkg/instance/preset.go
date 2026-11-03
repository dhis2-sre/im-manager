package instance

import (
	"context"
	"slices"

	"github.com/dhis2-sre/im-manager/internal/errdef"
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
	preset, err := s.instanceRepository.FindDeploymentById(ctx, id)
	if errdef.IsNotFound(err) || (err == nil && !preset.Preset) {
		return nil, errdef.NewNotFound("preset not found by id: %d", id)
	}
	if err != nil {
		return nil, err
	}

	return preset, nil
}

// EditPreset applies an edit to a preset. Nothing of a preset runs in the cluster, so a companion the edit switches off is deleted right away.
func (s Service) EditPreset(ctx context.Context, presetId uint, edit Edit) (*model.Deployment, error) {
	changes, err := s.EditDeployment(ctx, presetId, edit)
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

// CreateDeploymentFromPreset saves a new deployment holding the preset's instances with the edits applied. The preset's sensitive values carry over, which a client only ever sees masked.
func (s Service) CreateDeploymentFromPreset(ctx context.Context, preset, deployment *model.Deployment, edits map[string]InstanceEdit) error {
	decryptedPreset, err := s.decryptDeployment(preset)
	if err != nil {
		return err
	}

	if err := s.copyPresetInstances(decryptedPreset, deployment, edits); err != nil {
		return err
	}

	return s.SaveDeployments(ctx, deployment.GroupName, []*model.Deployment{deployment})
}

// copyPresetInstances gives the deployment the preset's instances with the edits applied. Consumed parameters are resolved again for the new name and group, and parameters the stack no longer declares are dropped.
func (s Service) copyPresetInstances(preset, deployment *model.Deployment, edits map[string]InstanceEdit) error {
	for _, presetInstance := range preset.Instances {
		instanceStack, err := s.stackService.Find(presetInstance.StackName)
		if err != nil {
			return err
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

	if err := s.rejectUnofferedStacks(deployment, edits); err != nil {
		return err
	}

	byStack := make(map[string]*model.DeploymentInstance, len(deployment.Instances))
	for _, deploymentInstance := range deployment.Instances {
		byStack[deploymentInstance.StackName] = deploymentInstance
	}
	if err := s.applyInstanceEdits(byStack, edits); err != nil {
		return err
	}

	added, switchedOff, err := s.reconcileCompanions(deployment, edits)
	if err != nil {
		return err
	}
	deployment.Instances = slices.DeleteFunc(deployment.Instances, func(deploymentInstance *model.DeploymentInstance) bool {
		return slices.Contains(switchedOff, deploymentInstance)
	})
	deployment.Instances = append(deployment.Instances, added...)

	return nil
}
