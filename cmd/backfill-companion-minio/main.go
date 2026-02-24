package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"gorm.io/gorm"
)

type candidate struct {
	GroupName      string
	DeploymentName string
	DeploymentID   uint
	InstanceName   string
	DatabaseID     string
}

type backfillResult struct {
	Added      int
	Candidates []candidate
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "backfill-companion-minio: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	dryRun := flag.Bool("dry-run", false, "show what would be changed without writing to the DB")
	flag.BoolVar(dryRun, "n", false, "short for -dry-run")
	flag.Parse()

	db, err := newDB(logger)
	if err != nil {
		return err
	}

	result, err := addMissingCompanionMinioInstances(context.Background(), db, *dryRun, logger)
	if err != nil {
		return err
	}

	if *dryRun {
		printReport(result.Candidates, logger)
		logger.Info("dry run completed", "minioInstancesWouldBeAdded", result.Added)
	} else {
		logger.Info("backfill completed", "minioInstancesAdded", result.Added)
		ids := uniqueSortedIDs(result.Candidates)
		if len(ids) > 0 {
			logger.Info(fmt.Sprintf("Deployment IDs to deploy: %s", formatIDs(ids)))
		}
	}
	return nil
}

func printReport(candidates []candidate, logger *slog.Logger) {
	if len(candidates) == 0 {
		logger.Info("No deployments would be updated.")
		return
	}
	byGroup := make(map[string][]candidate)
	for _, c := range candidates {
		byGroup[c.GroupName] = append(byGroup[c.GroupName], c)
	}
	groups := make([]string, 0, len(byGroup))
	for g := range byGroup {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	logger.Info("Deployments that would get a minio instance (by group):")
	for _, groupName := range groups {
		deploys := byGroup[groupName]
		sort.Slice(deploys, func(i, j int) bool { return deploys[i].DeploymentName < deploys[j].DeploymentName })
		logger.Info(fmt.Sprintf("Group: %s", groupName))
		for _, d := range deploys {
			logger.Info(fmt.Sprintf("  deployment %s (id %d) — would add minio instance %s", d.DeploymentName, d.DeploymentID, d.InstanceName))
		}
	}
	ids := uniqueSortedIDs(candidates)
	logger.Info(fmt.Sprintf("Deployment IDs that would need deploy (after backfill): %s", formatIDs(ids)))
}

func uniqueSortedIDs(candidates []candidate) []uint {
	seen := make(map[uint]struct{})
	for _, c := range candidates {
		seen[c.DeploymentID] = struct{}{}
	}
	ids := make([]uint, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func formatIDs(ids []uint) string {
	if len(ids) == 0 {
		return ""
	}
	s := fmt.Sprintf("%d", ids[0])
	for i := 1; i < len(ids); i++ {
		s += fmt.Sprintf(" %d", ids[i])
	}
	return s
}

func newDB(logger *slog.Logger) (*gorm.DB, error) {
	host, err := requireEnv("DATABASE_HOST")
	if err != nil {
		return nil, err
	}
	port, err := requireEnvAsInt("DATABASE_PORT")
	if err != nil {
		return nil, err
	}
	username, err := requireEnv("DATABASE_USERNAME")
	if err != nil {
		return nil, err
	}
	password, err := requireEnv("DATABASE_PASSWORD")
	if err != nil {
		return nil, err
	}
	name, err := requireEnv("DATABASE_NAME")
	if err != nil {
		return nil, err
	}
	return storage.NewDatabase(logger, storage.PostgresqlConfig{
		Host:         host,
		Port:         port,
		Username:     username,
		Password:     password,
		DatabaseName: name,
	})
}

func requireEnv(key string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("required environment variable %q not set", key)
	}
	return value, nil
}

func requireEnvAsInt(key string) (int, error) {
	valueStr, err := requireEnv(key)
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse environment variable %q as int: %w", key, err)
	}
	return value, nil
}

func addMissingCompanionMinioInstances(ctx context.Context, db *gorm.DB, dryRun bool, logger *slog.Logger) (backfillResult, error) {
	var deployments []model.Deployment
	err := db.WithContext(ctx).Preload("Instances.GormParameters").Find(&deployments).Error
	if err != nil {
		return backfillResult{}, fmt.Errorf("find deployments: %w", err)
	}
	var result backfillResult
	for i := range deployments {
		d := &deployments[i]
		r, err := backfillDeployment(ctx, db, d, dryRun, logger)
		if err != nil {
			return result, fmt.Errorf("deployment %d: %w", d.ID, err)
		}
		result.Added += r.Added
		result.Candidates = append(result.Candidates, r.Candidates...)
	}
	return result, nil
}

func backfillDeployment(ctx context.Context, db *gorm.DB, deployment *model.Deployment, dryRun bool, logger *slog.Logger) (backfillResult, error) {
	databaseID := getDatabaseIDFromDeployment(deployment)
	var result backfillResult
	for _, inst := range deployment.Instances {
		if inst.StackName != "dhis2-core" {
			continue
		}
		if getParamValue(inst, "STORAGE_TYPE") != "minio" {
			continue
		}
		if hasMinioInstance(deployment, inst.Name, inst.GroupName) {
			continue
		}

		c := candidate{
			GroupName:      deployment.GroupName,
			DeploymentName: deployment.Name,
			DeploymentID:   deployment.ID,
			InstanceName:   inst.Name,
			DatabaseID:     databaseID,
		}
		result.Candidates = append(result.Candidates, c)

		if dryRun {
			result.Added++
			continue
		}

		logger.Info(fmt.Sprintf("Adding minio instance for deployment %s (id %d)", deployment.Name, deployment.ID))
		if err := createMinioInstance(ctx, db, c); err != nil {
			return result, err
		}
		result.Added++
	}
	return result, nil
}

func createMinioInstance(ctx context.Context, db *gorm.DB, c candidate) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inst := &model.DeploymentInstance{
			Name:         c.InstanceName,
			GroupName:    c.GroupName,
			DeploymentID: c.DeploymentID,
			StackName:    "minio",
			Public:       false,
		}
		if err := tx.Create(inst).Error; err != nil {
			return fmt.Errorf("create minio instance: %w", err)
		}
		inst.Parameters = minioParameters(c.DatabaseID)
		if err := tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(inst).Error; err != nil {
			return fmt.Errorf("save minio instance parameters: %w", err)
		}
		return nil
	})
}

func minioParameters(databaseID string) model.DeploymentInstanceParameters {
	params := model.DeploymentInstanceParameters{
		"DATABASE_ID": {ParameterName: "DATABASE_ID", StackName: "minio", Value: databaseID},
	}
	for name, sp := range stack.MINIO.Parameters {
		if sp.Consumed || sp.DefaultValue == nil {
			continue
		}
		params[name] = model.DeploymentInstanceParameter{
			ParameterName: name,
			StackName:     "minio",
			Value:         *sp.DefaultValue,
		}
	}
	return params
}

func getParamValue(inst *model.DeploymentInstance, name string) string {
	if inst.Parameters != nil {
		if p, ok := inst.Parameters[name]; ok {
			return p.Value
		}
	}
	for _, p := range inst.GormParameters {
		if p.ParameterName == name {
			return p.Value
		}
	}
	return ""
}

func hasMinioInstance(deployment *model.Deployment, name, groupName string) bool {
	for _, inst := range deployment.Instances {
		if inst.StackName == "minio" && inst.Name == name && inst.GroupName == groupName {
			return true
		}
	}
	return false
}

func getDatabaseIDFromDeployment(deployment *model.Deployment) string {
	for _, inst := range deployment.Instances {
		if inst.StackName == "dhis2-db" {
			return getParamValue(inst, "DATABASE_ID")
		}
	}
	return ""
}
