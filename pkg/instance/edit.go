package instance

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
)

// Edit is a diff against a deployment rather than a snapshot of it: a nil field and an absent stack
// or parameter all mean unchanged. That is the only contract under which a client can send back a
// form it rendered, since reads replace sensitive values with ***.
type Edit struct {
	Description *string
	TTL         *uint
	// Instances are keyed by stack name. A deployment holds at most one instance per stack, and it
	// is the only key that can name a companion that does not exist yet.
	Instances map[string]InstanceEdit
}

// InstanceEdit is the change requested for the instance of one stack.
type InstanceEdit struct {
	Parameters Parameters
	Public     *bool
}

// DeploymentChanges is what an Edit resolved to: the deployment as it now stands, and the cluster
// work that is left.
type DeploymentChanges struct {
	Deployment *model.Deployment
	// Redeploy are the instances whose parameters changed plus the companions the edit added, in
	// deploy order. An instance the edit left alone is not in here, so editing pgAdmin does not roll
	// DHIS 2 core.
	Redeploy []*model.DeploymentInstance
	// Destroy are the companions whose gating parameter went false. Their rows outlive the edit
	// because destroying them needs the parameters they hold; the caller deletes them once the
	// cluster is rid of them.
	Destroy []*model.DeploymentInstance
}

// EditDeployment applies an edit to a deployment and returns the cluster work it implies. The edit is
// resolved and validated in full before anything is written, so a rejected edit changes neither the
// database nor the cluster.
func (s Service) EditDeployment(ctx context.Context, deploymentId uint, edit Edit) (*DeploymentChanges, error) {
	deployment, err := s.FindDecryptedDeploymentById(ctx, deploymentId)
	if err != nil {
		return nil, err
	}

	changes, err := s.planEdit(deployment, edit)
	if err != nil {
		return nil, err
	}

	if err := s.instanceRepository.SaveDeploymentDetails(ctx, deployment); err != nil {
		return nil, err
	}

	// Saving encrypts an instance's sensitive parameters in place, so it is the last thing done with
	// one here and the cluster work reloads its own decrypted copy.
	for _, deploymentInstance := range changes.Redeploy {
		deploymentStack, err := s.stackService.Find(deploymentInstance.StackName)
		if err != nil {
			return nil, err
		}
		if err := s.instanceRepository.SaveInstance(ctx, deploymentInstance, deploymentStack); err != nil {
			return nil, err
		}
	}

	return changes, nil
}

// planEdit resolves an edit against the deployment it is given, leaving the deployment holding the
// state the edit asks for and returning what that means for the cluster. It touches neither the
// database nor the cluster.
//
// A TTL, description or public change resolves to no cluster work at all. The inspector reads the
// TTL out of the database and the public flag only decides whether an instance is listed publicly,
// so neither is worth rolling DHIS 2 core for.
func (s Service) planEdit(deployment *model.Deployment, edit Edit) (*DeploymentChanges, error) {
	byStack := make(map[string]*model.DeploymentInstance, len(deployment.Instances))
	for _, deploymentInstance := range deployment.Instances {
		byStack[deploymentInstance.StackName] = deploymentInstance
	}

	offered, err := s.offeredCompanions(deployment.Instances)
	if err != nil {
		return nil, err
	}
	for stackName := range edit.Instances {
		if _, ok := byStack[stackName]; ok {
			continue
		}
		if _, ok := offered[stackName]; ok {
			continue
		}
		return nil, errdef.NewBadRequest("deployment %d has no instance of stack %q and none of its stacks offers it as a companion", deployment.ID, stackName)
	}

	before := s.resolvedParameterValues(deployment)

	if err := s.applyInstanceEdits(byStack, edit.Instances); err != nil {
		return nil, err
	}

	added, destroy, err := s.reconcileCompanions(deployment, edit.Instances)
	if err != nil {
		return nil, err
	}

	target := &model.Deployment{ID: deployment.ID, Name: deployment.Name, GroupName: deployment.GroupName, Group: deployment.Group}
	for _, deploymentInstance := range deployment.Instances {
		if !slices.Contains(destroy, deploymentInstance) {
			target.Instances = append(target.Instances, deploymentInstance)
		}
	}
	target.Instances = append(target.Instances, added...)

	order, err := s.DeploymentOrder(target)
	if err != nil {
		return nil, errdef.NewBadRequest("failed to validate the edited deployment: %v", err)
	}
	target.Instances = order

	if err := s.resolveParameters(target); err != nil {
		return nil, errdef.NewBadRequest("failed to resolve parameters: %v", err)
	}

	if edit.Description != nil {
		deployment.Description = *edit.Description
	}
	if edit.TTL != nil {
		deployment.TTL = *edit.TTL
	}
	// The instances the edit leaves behind are what the caller answers with. The ones on their way
	// out are gone as far as a client is concerned; their rows only outlive the answer because
	// destroying them needs the parameters they hold.
	deployment.Instances = order

	changes := &DeploymentChanges{Deployment: deployment, Destroy: destroy}
	for _, deploymentInstance := range order {
		if !slices.Contains(added, deploymentInstance) && maps.Equal(before[deploymentInstance.StackName], parameterValues(deploymentInstance)) {
			continue
		}
		changes.Redeploy = append(changes.Redeploy, deploymentInstance)
	}

	return changes, nil
}

// applyInstanceEdits merges the requested parameters into the instances that already exist.
func (s Service) applyInstanceEdits(byStack map[string]*model.DeploymentInstance, edits map[string]InstanceEdit) error {
	for stackName, deploymentInstance := range byStack {
		edit, ok := edits[stackName]
		if !ok {
			continue
		}

		if err := s.rejectConsumedParameters(stackName, maps.Keys(edit.Parameters)); err != nil {
			return errdef.NewBadRequest("%v", err)
		}
		if err := s.rejectImmutableParameters(deploymentInstance, edit.Parameters); err != nil {
			return err
		}

		for name, parameter := range edit.Parameters {
			deploymentInstance.Parameters[name] = model.DeploymentInstanceParameter{ParameterName: name, Value: parameter.Value}
		}

		if edit.Public != nil {
			deploymentInstance.Public = *edit.Public
		}
	}
	return nil
}

// reconcileCompanions derives which companions belong to the deployment from the parameters the edit
// leaves its instances with. The gating parameter is the gesture: a client turns pgAdmin on by
// setting ENABLE_PGADMIN, exactly as the deploy form does, rather than by adding an instance itself.
func (s Service) reconcileCompanions(deployment *model.Deployment, edits map[string]InstanceEdit) (added, destroy []*model.DeploymentInstance, err error) {
	byStack := make(map[string]*model.DeploymentInstance, len(deployment.Instances))
	for _, deploymentInstance := range deployment.Instances {
		byStack[deploymentInstance.StackName] = deploymentInstance
	}

	for _, host := range slices.Clone(deployment.Instances) {
		hostStack, err := s.stackService.Find(host.StackName)
		if err != nil {
			return nil, nil, err
		}

		for _, companion := range hostStack.Companions {
			existing, exists := byStack[companion.Stack.Name]

			switch {
			case companion.When.Matches(host.Parameters) && !exists:
				deploymentInstance, err := s.newCompanionInstance(deployment, host, companion.Stack.Name, edits[companion.Stack.Name])
				if err != nil {
					return nil, nil, err
				}
				byStack[companion.Stack.Name] = deploymentInstance
				added = append(added, deploymentInstance)
			case !companion.When.Matches(host.Parameters) && exists:
				destroy = append(destroy, existing)
			}
		}
	}

	return added, destroy, nil
}

func (s Service) newCompanionInstance(deployment *model.Deployment, host *model.DeploymentInstance, stackName string, edit InstanceEdit) (*model.DeploymentInstance, error) {
	companionStack, err := s.stackService.Find(stackName)
	if err != nil {
		return nil, err
	}

	parameters := make(model.DeploymentInstanceParameters, len(edit.Parameters))
	for name, parameter := range edit.Parameters {
		if companionStack.Parameters[name].Consumed {
			return nil, errdef.NewBadRequest("consumed parameters can't be supplied by the user: %s", name)
		}
		parameters[name] = model.DeploymentInstanceParameter{ParameterName: name, Value: parameter.Value}
	}

	return &model.DeploymentInstance{
		DeploymentID: deployment.ID,
		Name:         deployment.Name,
		Group:        host.Group,
		GroupName:    deployment.GroupName,
		StackName:    stackName,
		Parameters:   parameters,
	}, nil
}

// offeredCompanions names every stack the deployment's instances offer as a companion, whether or
// not its condition currently holds, so an edit may carry parameters for one it is turning on.
func (s Service) offeredCompanions(instances []*model.DeploymentInstance) (map[string]stack.Stack, error) {
	offered := map[string]stack.Stack{}
	for _, deploymentInstance := range instances {
		deploymentStack, err := s.stackService.Find(deploymentInstance.StackName)
		if err != nil {
			return nil, err
		}
		for _, companion := range deploymentStack.Companions {
			offered[companion.Stack.Name] = companion.Stack
		}
	}
	return offered, nil
}

// resolvedParameterValues is what the deployment's instances render with today, per stack. An
// instance renders with its whole resolved set rather than with the values a request names, so an
// instance that only consumes a parameter another instance changed is redeployed too. Resolving a
// deployment stored before its stacks moved on can fail, in which case the caller sees no values for
// it and redeploys everything, the safe reading of a state we cannot reason about.
func (s Service) resolvedParameterValues(deployment *model.Deployment) map[string]map[string]string {
	snapshot := &model.Deployment{ID: deployment.ID, Name: deployment.Name, GroupName: deployment.GroupName, Group: deployment.Group}
	for _, deploymentInstance := range deployment.Instances {
		clone := *deploymentInstance
		clone.Parameters = maps.Clone(deploymentInstance.Parameters)
		snapshot.Instances = append(snapshot.Instances, &clone)
	}

	if err := s.resolveParameters(snapshot); err != nil {
		return nil
	}

	values := make(map[string]map[string]string, len(snapshot.Instances))
	for _, deploymentInstance := range snapshot.Instances {
		values[deploymentInstance.StackName] = parameterValues(deploymentInstance)
	}
	return values
}

func parameterValues(instance *model.DeploymentInstance) map[string]string {
	values := make(map[string]string, len(instance.Parameters))
	for name, parameter := range instance.Parameters {
		values[name] = parameter.Value
	}
	return values
}

// DeleteDestroyedInstance removes the row of an instance the cluster is already rid of.
func (s Service) DeleteDestroyedInstance(ctx context.Context, instance *model.DeploymentInstance) error {
	if err := s.instanceRepository.DeleteDeploymentInstance(ctx, instance); err != nil {
		return fmt.Errorf("failed to delete instance %d: %v", instance.ID, err)
	}
	return nil
}
