package migratev3

import (
	"fmt"
	"maps"
	"slices"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"gorm.io/gorm"
)

// notDeployedYet is what a rewritten instance reports until the deploy phase has run. Failed rather
// than pending, because instance manager settles pending instances as abandoned when it starts.
const notDeployedYet = "migrated to dhis2-v2 and not deployed yet"

// Rewrite turns the deployment's rows into a dhis2-v2 instance seeded from the snapshot, plus a
// pgadmin companion when it had one. The dhis2-core row becomes the dhis2-v2 instance, keeping its
// id, so links and notifications pointing at it survive; the dhis2-db and minio rows go. It runs in
// one transaction together with the state change, so a deployment is either fully rewritten or
// untouched.
func Rewrite(db *gorm.DB, encryptionKey string, d LegacyDeployment, state *State) error {
	merged, err := MergedParameters(d)
	if err != nil {
		return err
	}
	merged["DATABASE_ID"] = fmt.Sprint(state.SnapshotDatabaseID)

	return db.Transaction(func(tx *gorm.DB) error {
		if err := replaceParameters(tx, encryptionKey, d.Core.ID, stack.DHIS2V2, merged); err != nil {
			return err
		}
		err := tx.Model(&model.DeploymentInstance{}).Where("id = ?", d.Core.ID).Updates(map[string]any{
			"stack_name":    stack.DHIS2V2.Name,
			"deploy_status": model.DeployStatusFailed,
			"deploy_error":  notDeployedYet,
			"deploy_log":    "",
		}).Error
		if err != nil {
			return fmt.Errorf("failed to rewrite instance %d: %v", d.Core.ID, err)
		}

		removed := []uint{d.Database.ID}
		if d.Minio != nil {
			removed = append(removed, d.Minio.ID)
		}
		// A user's lock on a database names the instance holding it, and the instances going away
		// are folded into the dhis2-v2 one.
		if err := tx.Model(&model.Lock{}).Where("instance_id IN ?", removed).Update("instance_id", d.Core.ID).Error; err != nil {
			return fmt.Errorf("failed to move database locks: %v", err)
		}
		if err := tx.Where("deployment_instance_id IN ?", removed).Delete(&model.DeploymentInstanceParameter{}).Error; err != nil {
			return fmt.Errorf("failed to delete the parameters of instances %v: %v", removed, err)
		}
		if err := tx.Delete(&model.DeploymentInstance{}, removed).Error; err != nil {
			return fmt.Errorf("failed to delete instances %v: %v", removed, err)
		}

		if d.PgAdmin != nil {
			if err := replaceParameters(tx, encryptionKey, d.PgAdmin.ID, stack.PgAdmin, PgAdminParameters(d)); err != nil {
				return err
			}
			err := tx.Model(&model.DeploymentInstance{}).Where("id = ?", d.PgAdmin.ID).Updates(map[string]any{
				"deploy_status": model.DeployStatusFailed,
				"deploy_error":  notDeployedYet,
				"deploy_log":    "",
			}).Error
			if err != nil {
				return fmt.Errorf("failed to rewrite instance %d: %v", d.PgAdmin.ID, err)
			}
		}

		state.Step = StepRewritten
		state.Error = ""
		return saveState(tx, state)
	})
}

func replaceParameters(tx *gorm.DB, encryptionKey string, instanceID uint, target stack.Stack, values map[string]string) error {
	if err := tx.Where("deployment_instance_id = ?", instanceID).Delete(&model.DeploymentInstanceParameter{}).Error; err != nil {
		return fmt.Errorf("failed to delete the parameters of instance %d: %v", instanceID, err)
	}

	parameters := make([]model.DeploymentInstanceParameter, 0, len(values))
	for _, name := range slices.Sorted(maps.Keys(values)) {
		value := values[name]
		if target.Parameters[name].Sensitive {
			encrypted, err := encryptText(encryptionKey, value)
			if err != nil {
				return fmt.Errorf("failed to encrypt parameter %s of instance %d: %v", name, instanceID, err)
			}
			value = encrypted
		}
		parameters = append(parameters, model.DeploymentInstanceParameter{
			DeploymentInstanceID: instanceID,
			ParameterName:        name,
			StackName:            target.Name,
			Value:                value,
		})
	}
	if err := tx.Create(&parameters).Error; err != nil {
		return fmt.Errorf("failed to save the parameters of instance %d: %v", instanceID, err)
	}
	return nil
}

// RestoreDatabaseID points the dhis2-v2 instance back at the database it was originally seeded
// from, once it has been seeded from the snapshot. A reset then behaves as it did before the
// migration, and the snapshot can be deleted. DATABASE_ID is not sensitive.
func RestoreDatabaseID(db *gorm.DB, state *State) error {
	err := db.Model(&model.DeploymentInstanceParameter{}).
		Where("deployment_instance_id = ? AND parameter_name = ?", state.CoreInstanceID, "DATABASE_ID").
		Update("value", state.OriginalDatabaseID).Error
	if err != nil {
		return fmt.Errorf("failed to restore DATABASE_ID of instance %d: %v", state.CoreInstanceID, err)
	}
	return nil
}
