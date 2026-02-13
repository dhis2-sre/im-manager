// One-off migration: updates DATABASE_HOSTNAME and DATABASE_ID for deployment instances
// after merging feat/unique-instance-names. Read the backup JSON from backup_group_dbs.py,
// then run with -dry-run first, then without to apply.
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
	DBInstanceID      uint   `json:"dbInstanceId"`
	DeploymentID      uint   `json:"deploymentId"`
	DeploymentName    string `json:"deploymentName"`
	SavedDatabaseID   uint   `json:"savedDatabaseId"`
	SavedDatabaseName string `json:"savedDatabaseName"`
}

// groupRow is used to read id and namespace from groups (after feat/unique-instance-names merge).
type groupRow struct {
	ID        uint   `gorm:"column:id"`
	Namespace string `gorm:"column:namespace"`
}

func main() {
	jsonPath := flag.String("json-file", "", "Path to backup JSON file (output of backup_group_dbs.py)")
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

	if err := runMigration(db, backup.Items, *dryRun, logger); err != nil {
		if *dryRun && errors.Is(err, errDryRun) {
			// Rollback was intentional
		} else {
			logger.Error("migration failed", "error", err)
			os.Exit(1)
		}
	}

	if *dryRun {
		logger.Info("dry run: rolled back (no changes made)")
	} else {
		logger.Info("migration completed")
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
		var inst model.DeploymentInstance
		if err := db.Where("id = ?", item.DBInstanceID).Select("id", "name", "group_name", "stack_name").First(&inst).Error; err != nil {
			return fmt.Errorf("instance id=%d: %w", item.DBInstanceID, err)
		}
		if inst.StackName != stackDhis2DB {
			return fmt.Errorf("instance id=%d is not dhis2-db (stack_name=%s)", item.DBInstanceID, inst.StackName)
		}

		var gr groupRow
		if err := db.Table("groups").Where("name = ?", inst.GroupName).Select("id", "namespace").First(&gr).Error; err != nil {
			return fmt.Errorf("group name=%q for instance id=%d: %w", inst.GroupName, item.DBInstanceID, err)
		}

		var count int64
		if err := db.Model(&model.Database{}).Where("id = ?", item.SavedDatabaseID).Count(&count).Error; err != nil {
			return fmt.Errorf("database id=%d: %w", item.SavedDatabaseID, err)
		}
		if count == 0 {
			return fmt.Errorf("database id=%d (savedDatabaseId) not found for instance id=%d", item.SavedDatabaseID, item.DBInstanceID)
		}
	}
	return nil
}

func runMigration(db *gorm.DB, items []backupItem, dryRun bool, logger *slog.Logger) error {
	// GORM commits when the callback returns nil; it rolls back on any returned error.
	return db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			var inst model.DeploymentInstance
			if err := tx.Where("id = ? AND stack_name = ?", item.DBInstanceID, stackDhis2DB).
				Select("id", "name", "group_name", "deployment_id", "stack_name").First(&inst).Error; err != nil {
				logger.Warn("skip: instance not found or not dhis2-db", "id", item.DBInstanceID)
				continue
			}

			var gr groupRow
			if err := tx.Table("groups").Where("name = ?", inst.GroupName).Select("id", "namespace").First(&gr).Error; err != nil {
				logger.Warn("skip: group not found", "group", inst.GroupName)
				continue
			}

			newHostname := fmt.Sprintf("%s-%d-database-postgresql.%s.svc", inst.Name, gr.ID, gr.Namespace)

			if dryRun {
				logger.Info("would set",
					"instance_id", inst.ID, "stack", inst.StackName,
					"DATABASE_ID", item.SavedDatabaseID,
					"DATABASE_HOSTNAME", newHostname)
			} else {
				res := tx.Model(&model.DeploymentInstanceParameter{}).
					Where("deployment_instance_id = ? AND parameter_name = ?", inst.ID, "DATABASE_ID").
					Update("value", strconv.FormatUint(uint64(item.SavedDatabaseID), 10))
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected > 0 {
					logger.Info("updated DATABASE_ID", "instance_id", inst.ID, "stack", inst.StackName, "value", item.SavedDatabaseID)
				}
				res = tx.Model(&model.DeploymentInstanceParameter{}).
					Where("deployment_instance_id = ? AND parameter_name = ?", inst.ID, "DATABASE_HOSTNAME").
					Update("value", newHostname)
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected > 0 {
					logger.Info("updated DATABASE_HOSTNAME", "instance_id", inst.ID, "stack", inst.StackName, "value", newHostname)
				}
			}

			type idStack struct {
				ID        uint   `gorm:"column:deployment_instance_id"`
				StackName string `gorm:"column:stack_name"`
			}
			var others []idStack
			if err := tx.Raw(
				`SELECT dip.deployment_instance_id AS deployment_instance_id, di.stack_name
				 FROM deployment_instance_parameters dip
				 INNER JOIN deployment_instances di ON di.id = dip.deployment_instance_id
				 WHERE di.deployment_id = ? AND dip.parameter_name = ? AND dip.deployment_instance_id != ?`,
				inst.DeploymentID, "DATABASE_HOSTNAME", inst.ID,
			).Scan(&others).Error; err != nil {
				return err
			}

			for _, o := range others {
				if dryRun {
					logger.Info("would set same deployment instance DATABASE_HOSTNAME",
						"instance_id", o.ID, "stack", o.StackName, "value", newHostname)
				} else {
					res := tx.Model(&model.DeploymentInstanceParameter{}).
						Where("deployment_instance_id = ? AND parameter_name = ?", o.ID, "DATABASE_HOSTNAME").
						Update("value", newHostname)
					if res.Error != nil {
						return res.Error
					}
					if res.RowsAffected > 0 {
						logger.Info("updated DATABASE_HOSTNAME (same deployment)", "instance_id", o.ID, "stack", o.StackName, "value", newHostname)
					}
				}
			}
		}

		if dryRun {
			return errDryRun // rollback: no changes committed
		}
		return nil // commit
	})
}
