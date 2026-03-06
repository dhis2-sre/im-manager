// One-off migration: after reset, restore each dhis2-db instance's DATABASE_ID
// parameter to the original (pre-save-as) database id. Read the backup JSON from
// backup_group_dbs.py (must include originalDatabaseId per item), then run with
// -dry-run first, then without to apply.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"gorm.io/gorm"
)

var errDryRun = errors.New("dry run rollback")

const stackDhis2DB = "dhis2-db"

type backupFile struct {
	Group string       `json:"group"`
	Items []backupItem `json:"items"`
}

type backupItem struct {
	DBInstanceID       uint        `json:"dbInstanceId"`
	DeploymentID       uint        `json:"deploymentId"`
	DeploymentName     string      `json:"deploymentName"`
	OriginalDatabaseID interface{} `json:"originalDatabaseId"` // numeric id or string slug
	SavedDatabaseID    uint        `json:"savedDatabaseId"`
	SavedDatabaseName  string      `json:"savedDatabaseName"`
}

func main() {
	jsonPath := flag.String("json-file", "", "Path to backup JSON file (output of backup_group_dbs.py, with originalDatabaseId)")
	dryRun := flag.Bool("dry-run", false, "Log planned updates and do not commit")
	flag.Parse()

	if *jsonPath == "" {
		fmt.Fprintf(os.Stderr, "missing -json-file\n")
		os.Exit(1)
	}

	logger := slog.Default()

	data, err := os.ReadFile(*jsonPath)
	if err != nil {
		logger.Error("failed to read JSON file", "path", *jsonPath, "error", err)
		os.Exit(1)
	}

	var backup backupFile
	if err := json.Unmarshal(data, &backup); err != nil {
		logger.Error("failed to parse JSON", "error", err)
		os.Exit(1)
	}

	if len(backup.Items) == 0 {
		logger.Info("no items in JSON; nothing to do")
		os.Exit(0)
	}

	db, err := openDB(logger)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	if err := validate(db, backup.Items); err != nil {
		logger.Error("validation failed", "error", err)
		os.Exit(1)
	}
	logger.Info("validation passed", "count", len(backup.Items))

	if err := runRestore(db, backup.Items, *dryRun, logger); err != nil {
		if *dryRun && errors.Is(err, errDryRun) {
			// Rollback was intentional
		} else {
			logger.Error("restore failed", "error", err)
			os.Exit(1)
		}
	}

	if *dryRun {
		logger.Info("dry run: rolled back (no changes made)")
	} else {
		logger.Info("restore completed")
	}
}

func openDB(logger *slog.Logger) (*gorm.DB, error) {
	host := getEnv("DATABASE_HOST", "")
	portStr := getEnv("DATABASE_PORT", "5432")
	user := getEnv("DATABASE_USERNAME", "")
	password := getEnv("DATABASE_PASSWORD", "")
	name := getEnv("DATABASE_NAME", "")

	if host == "" || user == "" || name == "" {
		return nil, fmt.Errorf("set DATABASE_HOST, DATABASE_USERNAME, DATABASE_NAME (and DATABASE_PASSWORD, DATABASE_PORT)")
	}
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 5432
	}

	return storage.NewDatabase(logger, storage.PostgresqlConfig{
		Host:         host,
		Port:         port,
		Username:     user,
		Password:     password,
		DatabaseName: name,
	})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func validate(db *gorm.DB, items []backupItem) error {
	for _, item := range items {
		if _, err := originalDatabaseIDToValue(item.OriginalDatabaseID); err != nil {
			return fmt.Errorf("instance id=%d: %w", item.DBInstanceID, err)
		}
		var inst model.DeploymentInstance
		if err := db.Where("id = ?", item.DBInstanceID).Select("id", "name", "group_name", "stack_name").First(&inst).Error; err != nil {
			return fmt.Errorf("instance id=%d: %w", item.DBInstanceID, err)
		}
		if inst.StackName != stackDhis2DB {
			return fmt.Errorf("instance id=%d is not dhis2-db (stack_name=%s)", item.DBInstanceID, inst.StackName)
		}
	}
	return nil
}

// originalDatabaseIDToValue returns the string to write to deployment_instance_parameters.value.
// originalDatabaseId in JSON can be a number (database id) or a string (slug e.g. "whoami-2-42-sql-gz").
func originalDatabaseIDToValue(v interface{}) (string, error) {
	if v == nil {
		return "", fmt.Errorf("missing originalDatabaseId in JSON (re-run backup_group_dbs.py to include it)")
	}
	switch val := v.(type) {
	case float64:
		if val < 0 || val != float64(uint64(val)) {
			return "", fmt.Errorf("originalDatabaseId must be a non-negative integer or string slug")
		}
		return strconv.FormatUint(uint64(val), 10), nil
	case string:
		if val == "" {
			return "", fmt.Errorf("originalDatabaseId string cannot be empty")
		}
		return val, nil
	default:
		return "", fmt.Errorf("originalDatabaseId must be number or string, got %T", v)
	}
}

func runRestore(db *gorm.DB, items []backupItem, dryRun bool, logger *slog.Logger) error {
	// GORM commits when the callback returns nil; it rolls back on any returned error.
	return db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			var inst model.DeploymentInstance
			if err := tx.Where("id = ? AND stack_name = ?", item.DBInstanceID, stackDhis2DB).
				Select("id", "name", "stack_name").First(&inst).Error; err != nil {
				logger.Warn("skip: instance not found or not dhis2-db", "id", item.DBInstanceID)
				continue
			}

			value, err := originalDatabaseIDToValue(item.OriginalDatabaseID)
			if err != nil {
				return err
			}
			if dryRun {
				logger.Info("would set DATABASE_ID to original",
					"instance_id", inst.ID, "stack", inst.StackName, "value", value)
				continue
			}
			res := tx.Model(&model.DeploymentInstanceParameter{}).
				Where("deployment_instance_id = ? AND parameter_name = ?", inst.ID, "DATABASE_ID").
				Update("value", value)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				logger.Info("restored DATABASE_ID", "instance_id", inst.ID, "stack", inst.StackName, "value", value)
			}
		}

		if dryRun {
			return errDryRun // rollback: no changes committed
		}
		return nil // commit
	})
}
