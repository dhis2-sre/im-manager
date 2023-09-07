package instance

import (
	"errors"
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB, config config.Config) *repository {
	return &repository{db, config}
}

type repository struct {
	db     *gorm.DB
	config config.Config
}

func (r repository) SaveChain(chain *model.Deployment) error {
	// TODO: Do we need the option to save nested entities... Yes, if we create the chain from a preset we need to store all the links as well
	//err := r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(chain).Error
	err := r.db.Create(&chain).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("a chain named %q already exists", chain.Name)
	}

	return err
}

func (r repository) FindChainById(id uint) (*model.Deployment, error) {
	var chain *model.Deployment
	err := r.db.
		Joins("Group").
		First(&chain, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("chain not found by id: %d", id)
	}

	return chain, err
}

func (r repository) SaveLink(link *model.DeploymentInstance) error {
	err := r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(link).Error
	// TODO: When is a link duplicated?
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("link already exists: %v", link)
	}

	return err
}

func (r repository) FindByIdDecrypted(id uint) (*model.Instance, error) {
	instance, err := r.FindById(id)
	if err != nil {
		return nil, err
	}

	err = decryptParameters(r.config.InstanceParameterEncryptionKey, instance)

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
	key := r.config.InstanceParameterEncryptionKey

	populateParameterRelations(instance)

	err := encryptParameters(key, instance)
	if err != nil {
		return err
	}

	err = r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(instance).Error
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

func encryptParameters(key string, instance *model.Instance) error {
	for i, parameter := range instance.Parameters {
		value, err := encryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		instance.Parameters[i].Value = value
	}

	return nil
}

func decryptParameters(key string, instance *model.Instance) error {
	for i, parameter := range instance.Parameters {
		value, err := decryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		instance.Parameters[i].Value = value
	}

	return nil
}
