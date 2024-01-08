package instance

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"golang.org/x/exp/maps"
	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB, instanceParameterEncryptionKey string) *repository {
	return &repository{db: db, instanceParameterEncryptionKey: instanceParameterEncryptionKey}
}

type repository struct {
	db                             *gorm.DB
	instanceParameterEncryptionKey string
}

func (r repository) DeleteDeploymentInstance(instance *model.DeploymentInstance) error {
	err := r.db.Unscoped().Delete(&model.DeploymentInstance{}, instance.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errdef.NewNotFound("instance not found by id: %d", instance.ID)
		}
		return fmt.Errorf("failed to delete instance %q: %v", instance.Name, err)
	}

	return nil
}

func (r repository) DeleteDeployment(deployment *model.Deployment) error {
	err := r.db.Unscoped().Delete(&model.Deployment{}, deployment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errdef.NewNotFound("deployment not found by id: %d", deployment.ID)
		}
		return fmt.Errorf("failed to delete deployment: %v", err)
	}

	return nil
}

func (r repository) SaveDeployment(deployment *model.Deployment) error {
	err := r.db.Create(&deployment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errdef.NewDuplicated("a deployment named %q already exists", deployment.Name)
		}
		return fmt.Errorf("failed to save deployment: %v", err)
	}

	return nil
}

func (r repository) FindDeploymentById(id uint) (*model.Deployment, error) {
	var deployment *model.Deployment
	err := r.db.
		Joins("Group").
		Joins("User").
		Preload("Instances.GormParameters").
		First(&deployment, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errdef.NewNotFound("deployment not found by id: %d", id)
		}
		return nil, fmt.Errorf("failed to find deployment: %v", err)
	}

	return deployment, nil
}

func (r repository) FindDeploymentInstanceById(id uint) (*model.DeploymentInstance, error) {
	var instance *model.DeploymentInstance
	err := r.db.
		Joins("Group").
		Preload("GormParameters").
		First(&instance, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errdef.NewNotFound("instance not found by id: %d", id)
		}
		return nil, fmt.Errorf("failed to find instance: %v", err)
	}

	return instance, nil
}

func (r repository) FindDecryptedDeploymentInstanceById(id uint) (*model.DeploymentInstance, error) {
	instance, err := r.FindDeploymentInstanceById(id)
	if err != nil {
		return nil, err
	}

	err = decryptParameters(r.instanceParameterEncryptionKey, instance)
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func (r repository) FindDecryptedDeploymentById(id uint) (*model.Deployment, error) {
	deployment, err := r.FindDeploymentById(id)
	if err != nil {
		return nil, err
	}

	for _, instance := range deployment.Instances {
		err := decryptParameters(r.instanceParameterEncryptionKey, instance)
		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}

func (r repository) SaveInstance(instance *model.DeploymentInstance) error {
	key := r.instanceParameterEncryptionKey

	err := encryptParameters(key, instance)
	if err != nil {
		return err
	}

	err = r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(instance).Error
	if err != nil {
		return fmt.Errorf("failed to save instance: %v", err)
	}
	return nil
}

func (r repository) FindByIdDecrypted(id uint) (*model.Instance, error) {
	instance, err := r.FindById(id)
	if err != nil {
		return nil, err
	}

	return instance, err
}

func (r repository) Link(source *model.Instance, destination *model.Instance) error {
	link := &model.Linked{
		SourceInstanceID:      source.ID,
		DestinationStackName:  destination.StackName,
		DestinationInstanceID: destination.ID,
	}

	err := r.db.Create(&link).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("instance (%d) already linked with a stack of type \"%s\"", source.ID, destination.StackName)
	}

	return err
}

func (r repository) Unlink(instance *model.Instance) error {
	link := &model.Linked{}

	// Does another instance depend on the instance we're trying to unlink
	err := r.db.First(link, "source_instance_id = ?", instance.ID).Error
	if err == nil {
		return fmt.Errorf("instance %d depends on %d", link.DestinationInstanceID, instance.ID)
	}

	// Any error beside ErrRecordNotFound?
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// Attempt to unlink
	err = r.db.Unscoped().Delete(link, "destination_instance_id = ?", instance.ID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

func (r repository) Save(instance *model.Instance) error {
	//key := r.instanceParameterEncryptionKey

	populateParameterRelations(instance)
	/*
		err := encryptParameters(key, instance)
		if err != nil {
			return err
		}
	*/
	err := r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(instance).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("instance named %q already exists", instance.Name)
	}

	return err
}

func (r repository) FindById(id uint) (*model.Instance, error) {
	var instance *model.Instance
	err := r.db.
		Joins("Group").
		Preload("Parameters").
		First(&instance, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("instance not found by id: %d", id)
	}

	return instance, err
}

func (r repository) FindByNameAndGroup(name, group string) (*model.Instance, error) {
	var instance *model.Instance

	err := r.db.
		Where("name = ?", name).
		Where("group_name = ?", group).
		First(&instance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("instance not found by name %q and group %q", name, group)
	}

	return instance, err
}

func (r repository) SaveDeployLog(instance *model.Instance, log string) error {
	return r.db.Model(&instance).Update("DeployLog", log).Error
}

func (r repository) SaveDeployLog_deployment(instance *model.DeploymentInstance, log string) error {
	err := r.db.Model(&instance).Update("DeployLog", log).Error
	if err != nil {
		return fmt.Errorf("failed to save deploy log: %v", err)
	}
	return nil
}

func (r repository) Delete(id uint) error {
	err := r.db.Unscoped().Delete(&model.Instance{}, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errdef.NewNotFound("instance not found by id: %d", id)
	}
	return err
}

const administratorGroupName = "administrators"

type GroupsWithInstances struct {
	Name      string            `json:"name"`
	Hostname  string            `json:"hostname"`
	Instances []*model.Instance `json:"instances"`
}

func (r repository) FindByGroups(groups []model.Group, presets bool) ([]GroupsWithInstances, error) {
	groupsByName := make(map[string]model.Group)
	for _, group := range groups {
		groupsByName[group.Name] = group
	}
	groupNames := maps.Keys(groupsByName)

	instances, err := r.findInstances(groupNames, presets)
	if err != nil {
		return nil, err
	}

	if len(instances) < 1 {
		return []GroupsWithInstances{}, nil
	}

	instancesByGroup := mapInstancesByGroup(groupNames, instances)

	return groupWithInstances(instancesByGroup, groupsByName), nil
}

func (r repository) findInstances(groupNames []string, presets bool) ([]*model.Instance, error) {
	query := r.db.
		Preload("Parameters")

	isAdmin := slices.Contains(groupNames, administratorGroupName)
	if !isAdmin {
		query = query.Where("group_name IN ?", groupNames)
	}

	var instances []*model.Instance
	err := query.
		Joins("User").
		Where("preset = ?", presets).
		Order("updated_at desc").
		Find(&instances).Error
	return instances, err
}

func (r repository) FindDeployments(groupNames []string) ([]*model.Deployment, error) {
	db := r.db

	isAdmin := slices.Contains(groupNames, administratorGroupName)
	if !isAdmin {
		db = db.Where("group_name IN ?", groupNames)
	}

	var deployments []*model.Deployment
	err := db.
		Joins("User").
		Preload("Instances").
		Order("updated_at desc").
		Find(&deployments).Error

	return deployments, err
}

func (r repository) FindPublicInstances() ([]GroupsWithInstances, error) {
	var instances []*model.Instance
	err := r.db.
		Joins("Group").
		Where("preset = false").
		Where("public = true").
		Find(&instances).Error
	if err != nil {
		return nil, err
	}

	if len(instances) < 1 {
		return []GroupsWithInstances{}, nil
	}

	groupsByName := make(map[string]model.Group)
	for _, instance := range instances {
		groupsByName[instance.Group.Name] = instance.Group
	}
	groupNames := maps.Keys(groupsByName)

	instancesByGroup := mapInstancesByGroup(groupNames, instances)

	return groupWithInstances(instancesByGroup, groupsByName), nil
}

func mapInstancesByGroup(groupNames []string, result []*model.Instance) map[string][]*model.Instance {
	instancesByGroup := make(map[string][]*model.Instance, len(groupNames))
	for _, instance := range result {
		groupName := instance.GroupName
		instancesByGroup[groupName] = append(instancesByGroup[groupName], instance)
	}
	return instancesByGroup
}

func groupWithInstances(instancesMap map[string][]*model.Instance, groupMap map[string]model.Group) []GroupsWithInstances {
	var groupWithInstances []GroupsWithInstances
	for groupName, instances := range instancesMap {
		if instances == nil {
			continue
		}
		group := groupMap[groupName]
		groupWithInstances = append(groupWithInstances, GroupsWithInstances{
			Name:      groupName,
			Hostname:  group.Hostname,
			Instances: instances,
		})
	}

	slices.SortFunc(groupWithInstances, func(a, b GroupsWithInstances) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return groupWithInstances
}

func populateParameterRelations(instance *model.Instance) {
	parameters := instance.Parameters
	if len(parameters) > 0 {
		for i := range parameters {
			parameters[i].InstanceID = instance.ID
		}
	}
}

func encryptParameters(key string, instance *model.DeploymentInstance) error {
	for i, parameter := range instance.Parameters {
		value, err := encryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		parameter.Value = value
		instance.Parameters[i] = parameter
	}

	return nil
}

func decryptParameters(key string, instance *model.DeploymentInstance) error {
	for i, parameter := range instance.Parameters {
		value, err := decryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		parameter.Value = value
		instance.Parameters[i] = parameter
	}

	return nil
}
