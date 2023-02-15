package instance

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/jackc/pgconn"
	"gorm.io/gorm"
	"k8s.io/utils/strings/slices"
)

func NewRepository(DB *gorm.DB, config config.Config) Repository {
	return &repository{db: DB, config: config}
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
	return instance, err
}

func (r repository) FindByNameAndGroup(instance string, group string) (*model.Instance, error) {
	var i *model.Instance

	err := r.db.
		Where("name = ?", instance).
		Where("group_name = ?", group).
		First(&i).Error
	if err != nil {
		return nil, err
	}

	return i, err
}

func (r repository) SaveDeployLog(instance *model.Instance, log string) error {
	return r.db.Model(&instance).Update("DeployLog", log).Error
}

func (r repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&model.Instance{}, id).Error
}

const AdministratorGroupName = "administrators"

func (r repository) FindByGroupNames_oold(names []string, presets bool) ([]*model.Instance, error) {
	query := r.db.
		Select("*").
		Preload("RequiredParameters.StackRequiredParameter").
		Preload("OptionalParameters.StackOptionalParameter")

	isAdmin := slices.Contains(names, AdministratorGroupName)
	if !isAdmin {
		query = query.Where("group_name IN ?", names)
	}

	//	var instances []*model.Instance
	var result []GroupWithInstances
	err := query.
		Where("preset = ?", presets).
		Group("group_name").
		Scan(&result).Error

	indent, _ := json.MarshalIndent(result, "", "  ")
	log.Println(string(indent))

	return nil, err
}

func (r repository) FindByGroupNames(names []string, presets bool) ([]*model.Instance, error) {
	var result []GroupWithInstances
	err := r.db.
		Model(&model.Instance{}).
		Where("group_name IN ?", names).
		Where("preset = ?", presets).
		Group("group_name").
		Scan(&result).Error

	indent, _ := json.MarshalIndent(result, "", "  ")
	log.Println(string(indent))

	return nil, err
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
