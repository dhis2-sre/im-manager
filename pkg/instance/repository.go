package instance

import (
	"errors"
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/jackc/pgconn"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

func NewRepository(db *gorm.DB, config config.Config) *repository {
	return &repository{db: db, config: config}
}

type repository struct {
	db     *gorm.DB
	config config.Config
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
	var perr *pgconn.PgError
	if errors.As(err, &perr) && perr.Code == "23505" {
		return fmt.Errorf("instance (%d) already linked with a stack of type \"%s\"", source.ID, destination.StackName)
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

	enrichParameters(instance)

	err := encryptParameters(key, instance)
	if err != nil {
		return err
	}

	return r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(instance).Error
}

func (r repository) FindById(id uint) (*model.Instance, error) {
	var instance *model.Instance
	err := r.db.
		Preload("RequiredParameters.StackRequiredParameter").
		Preload("OptionalParameters.StackOptionalParameter").
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

const AdministratorGroupName = "administrators"

type GroupWithInstances struct {
	Name      string            `json:"name"`
	Hostname  string            `json:"hostname"`
	Instances []*model.Instance `json:"instances"`
}

func (r repository) FindByGroups(groups []model.Group, presets bool) ([]GroupWithInstances, error) {
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
		return []GroupWithInstances{}, nil
	}

	instancesByGroup := mapInstancesByGroup(groupNames, instances)

	return groupWithInstances(instancesByGroup, groupsByName), nil
}

func (r repository) findInstances(groupNames []string, presets bool) ([]*model.Instance, error) {
	query := r.db.
		Preload("RequiredParameters.StackRequiredParameter").
		Preload("OptionalParameters.StackOptionalParameter")

	isAdmin := slices.Contains(groupNames, AdministratorGroupName)
	if !isAdmin {
		query = query.Where("group_name IN ?", groupNames)
	}

	var instances []*model.Instance
	err := query.
		Where("preset = ?", presets).
		Find(&instances).Error
	return instances, err
}

func mapInstancesByGroup(groupNames []string, result []*model.Instance) map[string][]*model.Instance {
	instancesByGroup := make(map[string][]*model.Instance, len(groupNames))
	for _, instance := range result {
		groupName := instance.GroupName
		instancesByGroup[groupName] = append(instancesByGroup[groupName], instance)
	}
	return instancesByGroup
}

func groupWithInstances(instancesMap map[string][]*model.Instance, groupMap map[string]model.Group) []GroupWithInstances {
	var groupWithInstances []GroupWithInstances
	for groupName, instances := range instancesMap {
		if instances == nil {
			continue
		}
		group := groupMap[groupName]
		groupWithInstances = append(groupWithInstances, GroupWithInstances{
			Name:      groupName,
			Hostname:  group.Hostname,
			Instances: instances,
		})
	}
	return groupWithInstances
}

// TODO: Rename PopulateRelations? Or something else?
func enrichParameters(instance *model.Instance) {
	requiredParameters := instance.RequiredParameters
	if len(requiredParameters) > 0 {
		for i := range requiredParameters {
			requiredParameters[i].InstanceID = instance.ID
			requiredParameters[i].StackName = instance.StackName
		}
	}

	optionalParameters := instance.OptionalParameters
	if len(optionalParameters) > 0 {
		for i := range optionalParameters {
			optionalParameters[i].InstanceID = instance.ID
			optionalParameters[i].StackName = instance.StackName
		}
	}
}

func encryptParameters(key string, instance *model.Instance) error {
	for i, parameter := range instance.RequiredParameters {
		value, err := encryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		instance.RequiredParameters[i].Value = value
	}

	for i, parameter := range instance.OptionalParameters {
		value, err := encryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		instance.OptionalParameters[i].Value = value
	}

	return nil
}

func decryptParameters(key string, instance *model.Instance) error {
	for i, parameter := range instance.RequiredParameters {
		value, err := decryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		instance.RequiredParameters[i].Value = value
	}

	for i, parameter := range instance.OptionalParameters {
		value, err := decryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		instance.OptionalParameters[i].Value = value
	}

	return nil
}
