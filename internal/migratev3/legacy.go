// Package migratev3 moves deployments made of the stacks version 3.0 removed (dhis2-db, minio,
// dhis2-core and pgadmin on top of them) onto the dhis2-v2 umbrella stack. It backs the one-off
// cmd/migrate-v3 command and goes away together with it once production has been migrated.
package migratev3

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"gorm.io/gorm"
)

const (
	legacyDatabaseStack = "dhis2-db"
	legacyMinioStack    = "minio"
	legacyCoreStack     = "dhis2-core"
	pgAdminStack        = "pgadmin"
)

// legacySensitive is what the removed stacks declared sensitive, which decides which stored values
// are ciphertext. It matches the frozen lists in migration 20260515000.
var legacySensitive = map[string]map[string]bool{
	legacyDatabaseStack: {"DATABASE_PASSWORD": true, "DATABASE_USERNAME": true},
	legacyMinioStack:    {},
	legacyCoreStack: {
		"DATABASE_PASSWORD":          true,
		"DATABASE_USERNAME":          true,
		"S3_REGION":                  true,
		"S3_IDENTITY":                true,
		"S3_SECRET":                  true,
		"FILESYSTEM_VOLUME_SIZE":     true,
		"CUSTOM_DHIS2_CONFIG":        true,
		"GOOGLE_AUTH_PROJECT_ID":     true,
		"GOOGLE_AUTH_PRIVATE_KEY":    true,
		"GOOGLE_AUTH_PRIVATE_KEY_ID": true,
		"GOOGLE_AUTH_CLIENT_EMAIL":   true,
		"GOOGLE_AUTH_CLIENT_ID":      true,
	},
	pgAdminStack: {"PGADMIN_USERNAME": true, "PGADMIN_PASSWORD": true, "DATABASE_USERNAME": true},
}

// removedStacks are legacy stacks this migration does not convert. Production has none of them, so
// finding one stops the run instead of guessing.
var removedStacks = []string{"dhis2", "im-job-runner", "chap-core", "chap-db", "chap-valkey", "chap-worker"}

// LegacyDeployment is a deployment still made of the removed stacks, with its parameters decrypted.
type LegacyDeployment struct {
	Deployment *model.Deployment
	Group      *model.Group
	Core       *model.DeploymentInstance
	Database   *model.DeploymentInstance
	Minio      *model.DeploymentInstance
	PgAdmin    *model.DeploymentInstance
}

// ReleaseName is the helm release name every instance of the deployment is derived from.
func (d LegacyDeployment) ReleaseName() string {
	return fmt.Sprintf("%s-%d", d.Deployment.Name, d.Group.ID)
}

func (d LegacyDeployment) StorageType() string {
	return d.Core.Parameters["STORAGE_TYPE"].Value
}

// Unsupported is a deployment the migration refuses to touch, with the reason.
type Unsupported struct {
	DeploymentID   uint
	DeploymentName string
	GroupName      string
	Reason         string
}

// FindLegacyDeployments loads every deployment that still has an instance of a removed stack. It
// reads through plain preloads only, because it also runs against the schema of the version before
// 3.0, where joining the deployments table would select columns that do not exist yet.
func FindLegacyDeployments(db *gorm.DB, encryptionKey string) ([]LegacyDeployment, []Unsupported, error) {
	var deployments []*model.Deployment
	err := db.
		Preload("Group.Cluster").
		Preload("Instances.GormParameters").
		Order("id").
		Find(&deployments).Error
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load deployments: %v", err)
	}

	var legacy []LegacyDeployment
	var unsupported []Unsupported
	for _, deployment := range deployments {
		if !hasLegacyInstance(deployment) {
			continue
		}
		candidate, reason, err := classify(deployment, encryptionKey)
		if err != nil {
			return nil, nil, fmt.Errorf("deployment %d (%s): %v", deployment.ID, deployment.Name, err)
		}
		if reason != "" {
			unsupported = append(unsupported, Unsupported{deployment.ID, deployment.Name, deployment.GroupName, reason})
			continue
		}
		legacy = append(legacy, candidate)
	}
	return legacy, unsupported, nil
}

func hasLegacyInstance(deployment *model.Deployment) bool {
	for _, instance := range deployment.Instances {
		if _, ok := legacySensitive[instance.StackName]; ok && instance.StackName != pgAdminStack {
			return true
		}
		if slices.Contains(removedStacks, instance.StackName) {
			return true
		}
	}
	return false
}

func classify(deployment *model.Deployment, encryptionKey string) (LegacyDeployment, string, error) {
	candidate := LegacyDeployment{Deployment: deployment, Group: deployment.Group}
	var stacks []string
	for _, instance := range deployment.Instances {
		stacks = append(stacks, instance.StackName)
		if err := decryptInstance(encryptionKey, instance, legacySensitive[instance.StackName]); err != nil {
			return LegacyDeployment{}, "", err
		}
		switch instance.StackName {
		case legacyCoreStack:
			candidate.Core = instance
		case legacyDatabaseStack:
			candidate.Database = instance
		case legacyMinioStack:
			candidate.Minio = instance
		case pgAdminStack:
			candidate.PgAdmin = instance
		default:
			return LegacyDeployment{}, fmt.Sprintf("stack %q is not migrated", instance.StackName), nil
		}
	}
	sort.Strings(stacks)

	switch {
	case candidate.Group == nil:
		return LegacyDeployment{}, "the deployment's group does not exist", nil
	case candidate.Core == nil || candidate.Database == nil:
		return LegacyDeployment{}, fmt.Sprintf("needs both dhis2-core and dhis2-db, has %s", strings.Join(stacks, ", ")), nil
	case (candidate.StorageType() == "minio") != (candidate.Minio != nil):
		return LegacyDeployment{}, fmt.Sprintf("storage type %q does not match its instances %s", candidate.StorageType(), strings.Join(stacks, ", ")), nil
	case candidate.Core.Parameters["DEPLOY_CHAP"].Value == "true":
		return LegacyDeployment{}, "DEPLOY_CHAP is enabled, and chap is not migrated", nil
	}
	return candidate, "", nil
}

// renamed maps a removed stack's parameter onto its dhis2-v2 name. Parameters missing from both
// renamed and dropped keep their name, and must then exist on dhis2-v2.
var renamed = map[string]map[string]string{
	legacyCoreStack: {
		"RESOURCES_REQUESTS_CPU":    "CORE_RESOURCES_REQUESTS_CPU",
		"RESOURCES_REQUESTS_MEMORY": "CORE_RESOURCES_REQUESTS_MEMORY",
	},
	legacyDatabaseStack: {
		"RESOURCES_REQUESTS_CPU":    "DB_RESOURCES_REQUESTS_CPU",
		"RESOURCES_REQUESTS_MEMORY": "DB_RESOURCES_REQUESTS_MEMORY",
	},
}

// dropped are the parameters dhis2-v2 has no equivalent for. Chart versions belong to the charts
// the removed stacks deployed, and dhis2-core's copies of the database parameters are the ones it
// consumed from dhis2-db, whose own values are taken instead.
var dropped = map[string]map[string]bool{
	legacyCoreStack: {
		"CHART_VERSION":     true,
		"ALLOW_SUSPEND":     true,
		"DEPLOY_GLOWROOT":   true,
		"DATABASE_HOSTNAME": true,
		"DATABASE_NAME":     true,
		"DATABASE_PASSWORD": true,
		"DATABASE_USERNAME": true,
	},
	legacyDatabaseStack: {
		"CHART_VERSION":    true,
		"DATABASE_VERSION": true,
	},
	legacyMinioStack: {
		"MINIO_CHART_VERSION": true,
		"IMAGE_PULL_POLICY":   true,
		"DATABASE_ID":         true,
	},
}

// MergedParameters builds the plaintext parameters of the dhis2-v2 instance replacing the
// deployment's dhis2-core, dhis2-db and minio instances. Every dhis2-v2 parameter gets a value,
// since helmfile only sees stored parameters, and a parameter the mapping does not know about is an
// error rather than something silently lost.
func MergedParameters(d LegacyDeployment) (map[string]string, error) {
	target := stack.DHIS2V2.Parameters
	merged := map[string]string{}
	for name, parameter := range target {
		if parameter.DefaultValue != nil {
			merged[name] = *parameter.DefaultValue
		}
	}

	var errs []string
	apply := func(instance *model.DeploymentInstance) {
		if instance == nil {
			return
		}
		for name, parameter := range instance.Parameters {
			if dropped[instance.StackName][name] {
				continue
			}
			targetName := name
			if renamedName, ok := renamed[instance.StackName][name]; ok {
				targetName = renamedName
			}
			if _, ok := target[targetName]; !ok {
				errs = append(errs, fmt.Sprintf("%s parameter %s has no dhis2-v2 equivalent", instance.StackName, name))
				continue
			}
			merged[targetName] = trimmed(parameter.Value, legacySensitive[instance.StackName][name])
		}
	}
	// The database instance goes last so its values win over the copies dhis2-core consumed.
	apply(d.Core)
	apply(d.Minio)
	apply(d.Database)

	merged["ENABLE_PGADMIN"] = fmt.Sprint(d.PgAdmin != nil)

	for name, parameter := range target {
		value, ok := merged[name]
		if !ok {
			errs = append(errs, fmt.Sprintf("dhis2-v2 parameter %s has no value", name))
			continue
		}
		if parameter.Validator != nil {
			if err := parameter.Validator(value); err != nil {
				errs = append(errs, fmt.Sprintf("dhis2-v2 parameter %s: %v", name, err))
			}
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return merged, nil
}

// trimmed drops the whitespace around a single-line value: none of them means it, and around an image
// tag it makes every pod invalid. A value of only whitespace stays, since " " is how an empty optional
// parameter is stored, and sensitive or multi-line values, such as keys and custom config, are kept.
func trimmed(value string, sensitive bool) string {
	if sensitive || strings.Contains(value, "\n") {
		return value
	}
	if trimmedValue := strings.TrimSpace(value); trimmedValue != "" {
		return trimmedValue
	}
	return value
}

// PgAdminParameters builds the plaintext parameters of the pgadmin companion once its database is
// the CloudNativePG cluster of the dhis2-v2 instance. Consumed values are stored when a deployment
// is saved rather than resolved at deploy time, so the new hostname has to be written here.
func PgAdminParameters(d LegacyDeployment) map[string]string {
	defaults := stack.PgAdmin.Parameters
	return map[string]string{
		"PGADMIN_USERNAME":  d.PgAdmin.Parameters["PGADMIN_USERNAME"].Value,
		"PGADMIN_PASSWORD":  d.PgAdmin.Parameters["PGADMIN_PASSWORD"].Value,
		"CHART_VERSION":     *defaults["CHART_VERSION"].DefaultValue,
		"DATABASE_HOSTNAME": fmt.Sprintf("%s-dhis2-postgresql-rw.%s.svc", d.ReleaseName(), d.Group.Namespace),
		"DATABASE_NAME":     d.Database.Parameters["DATABASE_NAME"].Value,
		"DATABASE_USERNAME": d.Database.Parameters["DATABASE_USERNAME"].Value,
	}
}
