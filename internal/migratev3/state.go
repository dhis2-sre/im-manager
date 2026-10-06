package migratev3

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Step is the last step a deployment's migration completed. Each phase of the command moves a
// deployment from one step to the next and skips the deployments already past it, so any phase can
// be run again after a failure.
type Step string

const (
	// StepNone: nothing has happened yet, the deployment is still made of the removed stacks.
	StepNone Step = ""
	// StepSnapshotted: the database and file store were saved as a database record, and the old
	// core no longer runs.
	StepSnapshotted Step = "snapshotted"
	// StepRewritten: the deployment's rows describe a dhis2-v2 instance seeded from the snapshot.
	StepRewritten Step = "rewritten"
	// StepReleased: the old releases that collide with dhis2-v2 are gone, and the old database is
	// scaled to zero and kept as a fallback.
	StepReleased Step = "released"
	// StepDeployed: dhis2-v2 is running and seeded, DATABASE_ID points at the original database
	// again and the old database release is gone.
	StepDeployed Step = "deployed"
	// StepCleaned: the snapshot has been deleted.
	StepCleaned Step = "cleaned"
)

// State records one deployment's migration. The table is created by the command and dropped
// together with it.
type State struct {
	DeploymentID       uint `gorm:"primaryKey"`
	DeploymentName     string
	GroupName          string
	CoreInstanceID     uint
	PgAdminInstanceID  uint
	HasMinio           bool
	OriginalDatabaseID string
	SnapshotDatabaseID uint
	// WasPaused is restored once dhis2-v2 runs: a paused instance had its database scaled down.
	WasPaused bool
	Step      Step
	Error     string `gorm:"type:text"`
	UpdatedAt time.Time
}

func (State) TableName() string {
	return "v3_migration_states"
}

func migrateStateTable(db *gorm.DB) error {
	return db.AutoMigrate(&State{})
}

func findState(db *gorm.DB, deploymentID uint) (*State, error) {
	var state State
	err := db.First(&state, deploymentID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load the migration state of deployment %d: %v", deploymentID, err)
	}
	return &state, nil
}

func findStates(db *gorm.DB, step Step) ([]State, error) {
	var states []State
	if err := db.Where("step = ?", step).Order("deployment_id").Find(&states).Error; err != nil {
		return nil, fmt.Errorf("failed to load migration states: %v", err)
	}
	return states, nil
}

func saveState(db *gorm.DB, state *State) error {
	if err := db.Save(state).Error; err != nil {
		return fmt.Errorf("failed to save the migration state of deployment %d: %v", state.DeploymentID, err)
	}
	return nil
}

func recordFailure(db *gorm.DB, state *State, failure error) error {
	state.Error = failure.Error()
	return errors.Join(failure, saveState(db, state))
}
