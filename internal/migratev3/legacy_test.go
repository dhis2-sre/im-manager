package migratev3

import (
	"maps"
	"slices"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The parameters the removed stacks stored, with the defaults they had on the last release before
// version 3.0.
func legacyDatabaseParameters() map[string]string {
	return map[string]string{
		"DATABASE_ID":               "42",
		"DATABASE_SIZE":             "30Gi",
		"DATABASE_NAME":             "dhis2",
		"DATABASE_PASSWORD":         "secret",
		"DATABASE_USERNAME":         "dhis",
		"DATABASE_VERSION":          "16",
		"RESOURCES_REQUESTS_CPU":    "500m",
		"RESOURCES_REQUESTS_MEMORY": "512Mi",
		"CHART_VERSION":             "16.4.5",
	}
}

func legacyMinioParameters() map[string]string {
	return map[string]string{
		"MINIO_STORAGE_SIZE":  "16Gi",
		"MINIO_CHART_VERSION": "14.7.5",
		"IMAGE_PULL_POLICY":   "IfNotPresent",
		"DATABASE_ID":         "42",
	}
}

func legacyCoreParameters() map[string]string {
	return map[string]string{
		"IMAGE_TAG":                       "2.41.3",
		"IMAGE_REPOSITORY":                "core",
		"IMAGE_PULL_POLICY":               "Always",
		"STORAGE_TYPE":                    "minio",
		"S3_BUCKET":                       "dhis2",
		"S3_REGION":                       "eu-west-1",
		"S3_IDENTITY":                     "-",
		"S3_SECRET":                       "-",
		"DHIS2_HOME":                      "/opt/dhis2",
		"FLYWAY_MIGRATE_OUT_OF_ORDER":     "false",
		"FLYWAY_REPAIR_BEFORE_MIGRATION":  "false",
		"RESOURCES_REQUESTS_CPU":          "1",
		"RESOURCES_REQUESTS_MEMORY":       "3Gi",
		"MIN_READY_SECONDS":               "5",
		"LIVENESS_PROBE_TIMEOUT_SECONDS":  "1",
		"READINESS_PROBE_TIMEOUT_SECONDS": "1",
		"STARTUP_PROBE_FAILURE_THRESHOLD": "60",
		"STARTUP_PROBE_PERIOD_SECONDS":    "5",
		"JAVA_OPTS":                       " ",
		"CHART_VERSION":                   "0.34.11",
		"ENABLE_QUERY_LOGGING":            "false",
		"FILESYSTEM_VOLUME_SIZE":          "8Gi",
		"SAME_SITE_COOKIES":               "lax",
		"CUSTOM_DHIS2_CONFIG":             " ",
		"ALLOW_SUSPEND":                   "true",
		"DEPLOY_GLOWROOT":                 "false",
		"DEPLOY_CHAP":                     "false",
		"GOOGLE_AUTH_PROJECT_ID":          " ",
		"GOOGLE_AUTH_PRIVATE_KEY":         " ",
		"GOOGLE_AUTH_PRIVATE_KEY_ID":      " ",
		"GOOGLE_AUTH_CLIENT_EMAIL":        " ",
		"GOOGLE_AUTH_CLIENT_ID":           " ",
		"DATABASE_HOSTNAME":               "play-7-database-postgresql.play.svc",
		"DATABASE_NAME":                   "dhis2",
		"DATABASE_PASSWORD":               "secret",
		"DATABASE_USERNAME":               "dhis",
	}
}

func legacyPgAdminParameters() map[string]string {
	return map[string]string{
		"PGADMIN_USERNAME":  "admin@dhis2.org",
		"PGADMIN_PASSWORD":  "pgadmin",
		"CHART_VERSION":     "1.30.0",
		"DATABASE_HOSTNAME": "play-7-database-postgresql.play.svc",
		"DATABASE_NAME":     "dhis2",
		"DATABASE_USERNAME": "dhis",
	}
}

func instance(id uint, stackName string, values map[string]string) *model.DeploymentInstance {
	parameters := model.DeploymentInstanceParameters{}
	for name, value := range values {
		parameters[name] = model.DeploymentInstanceParameter{ParameterName: name, StackName: stackName, Value: value}
	}
	return &model.DeploymentInstance{ID: id, Name: "play", StackName: stackName, Parameters: parameters}
}

func legacyDeployment() LegacyDeployment {
	return LegacyDeployment{
		Deployment: &model.Deployment{ID: 3, Name: "play", GroupName: "play"},
		Group:      &model.Group{ID: 7, Name: "play", Namespace: "play"},
		Database:   instance(1, legacyDatabaseStack, legacyDatabaseParameters()),
		Minio:      instance(2, legacyMinioStack, legacyMinioParameters()),
		Core:       instance(3, legacyCoreStack, legacyCoreParameters()),
	}
}

func TestMergedParameters(t *testing.T) {
	t.Run("covers every dhis2-v2 parameter", func(t *testing.T) {
		merged, err := MergedParameters(legacyDeployment())
		require.NoError(t, err)

		assert.ElementsMatch(t, keys(stack.DHIS2V2.Parameters), keys(merged))
	})

	t.Run("keeps the values the deployment had", func(t *testing.T) {
		merged, err := MergedParameters(legacyDeployment())
		require.NoError(t, err)

		assert.Equal(t, "2.41.3", merged["IMAGE_TAG"])
		assert.Equal(t, "Always", merged["IMAGE_PULL_POLICY"], "dhis2-core's pull policy, not minio's")
		assert.Equal(t, "42", merged["DATABASE_ID"])
		assert.Equal(t, "30Gi", merged["DATABASE_SIZE"])
		assert.Equal(t, "secret", merged["DATABASE_PASSWORD"])
		assert.Equal(t, "16Gi", merged["MINIO_STORAGE_SIZE"])
		assert.Equal(t, "60", merged["STARTUP_PROBE_FAILURE_THRESHOLD"])
	})

	t.Run("renames the resource requests per component", func(t *testing.T) {
		merged, err := MergedParameters(legacyDeployment())
		require.NoError(t, err)

		assert.Equal(t, "1", merged["CORE_RESOURCES_REQUESTS_CPU"])
		assert.Equal(t, "3Gi", merged["CORE_RESOURCES_REQUESTS_MEMORY"])
		assert.Equal(t, "500m", merged["DB_RESOURCES_REQUESTS_CPU"])
		assert.Equal(t, "512Mi", merged["DB_RESOURCES_REQUESTS_MEMORY"])
	})

	t.Run("takes the chart version and new parameters from dhis2-v2", func(t *testing.T) {
		merged, err := MergedParameters(legacyDeployment())
		require.NoError(t, err)

		assert.Equal(t, *stack.DHIS2V2.Parameters["CHART_VERSION"].DefaultValue, merged["CHART_VERSION"])
		assert.Equal(t, "false", merged["ENABLE_DORIS"])
		assert.Equal(t, "false", merged["ENABLE_PGADMIN"])
	})

	t.Run("enables pgadmin when the deployment had it", func(t *testing.T) {
		d := legacyDeployment()
		d.PgAdmin = instance(4, pgAdminStack, legacyPgAdminParameters())

		merged, err := MergedParameters(d)
		require.NoError(t, err)

		assert.Equal(t, "true", merged["ENABLE_PGADMIN"])
	})

	t.Run("refuses a parameter it does not know", func(t *testing.T) {
		d := legacyDeployment()
		d.Core.Parameters["SOMETHING_NEW"] = model.DeploymentInstanceParameter{Value: "x"}

		_, err := MergedParameters(d)

		assert.ErrorContains(t, err, "dhis2-core parameter SOMETHING_NEW has no dhis2-v2 equivalent")
	})

	t.Run("refuses a deployment without a database to seed from", func(t *testing.T) {
		d := legacyDeployment()
		delete(d.Database.Parameters, "DATABASE_ID")

		_, err := MergedParameters(d)

		assert.ErrorContains(t, err, "dhis2-v2 parameter DATABASE_ID has no value")
	})
}

func TestPgAdminParameters(t *testing.T) {
	d := legacyDeployment()
	d.PgAdmin = instance(4, pgAdminStack, legacyPgAdminParameters())

	parameters := PgAdminParameters(d)

	assert.Equal(t, "play-7-dhis2-postgresql-rw.play.svc", parameters["DATABASE_HOSTNAME"])
	assert.Equal(t, "admin@dhis2.org", parameters["PGADMIN_USERNAME"])
	assert.Equal(t, *stack.PgAdmin.Parameters["CHART_VERSION"].DefaultValue, parameters["CHART_VERSION"])
	assert.ElementsMatch(t, keys(stack.PgAdmin.Parameters), keys(parameters))
}

func TestClassify(t *testing.T) {
	deployment := func(name string, instances ...*model.DeploymentInstance) *model.Deployment {
		for _, instance := range instances {
			seal(t, instance)
		}
		return &model.Deployment{ID: 3, Name: name, Group: &model.Group{ID: 7, Name: "play"}, Instances: instances}
	}

	t.Run("accepts the stacks it migrates and decrypts their parameters", func(t *testing.T) {
		d := legacyDeployment()

		candidate, reason, err := classify(deployment("play", d.Database, d.Minio, d.Core), testKey)

		require.NoError(t, err)
		assert.Empty(t, reason)
		assert.Equal(t, "secret", candidate.Database.Parameters["DATABASE_PASSWORD"].Value)
		assert.Equal(t, "8Gi", candidate.Core.Parameters["FILESYSTEM_VOLUME_SIZE"].Value)
	})

	tests := map[string]struct {
		deployment *model.Deployment
		reason     string
	}{
		"a stack it does not migrate": {
			deployment: deployment("play", instance(1, "chap-core", nil)),
			reason:     `stack "chap-core" is not migrated`,
		},
		"a database without a core": {
			deployment: deployment("play", instance(1, legacyDatabaseStack, legacyDatabaseParameters())),
			reason:     "needs both dhis2-core and dhis2-db, has dhis2-db",
		},
		"minio storage without the minio instance": {
			deployment: deployment("play", instance(1, legacyDatabaseStack, legacyDatabaseParameters()), instance(3, legacyCoreStack, legacyCoreParameters())),
			reason:     `storage type "minio" does not match its instances dhis2-core, dhis2-db`,
		},
		"a release name containing dhis2": {
			deployment: deployment("dhis2-play", legacyDeployment().Database, legacyDeployment().Minio, legacyDeployment().Core),
			reason:     `release "dhis2-play-7" contains "dhis2", which dhis2-v2 does not resolve the database of`,
		},
	}
	for name, test := range tests {
		t.Run("refuses "+name, func(t *testing.T) {
			_, reason, err := classify(test.deployment, testKey)

			require.NoError(t, err)
			assert.Equal(t, test.reason, reason)
		})
	}
}

func keys[V any](m map[string]V) []string {
	return slices.Collect(maps.Keys(m))
}

const testKey = "0123456789abcdef0123456789abcdef"

// seal encrypts the instance's sensitive parameters the way they are stored.
func seal(t *testing.T, instance *model.DeploymentInstance) {
	t.Helper()
	for name, parameter := range instance.Parameters {
		if !legacySensitive[instance.StackName][name] {
			continue
		}
		value, err := encryptText(testKey, parameter.Value)
		require.NoError(t, err)
		parameter.Value = value
		instance.Parameters[name] = parameter
	}
}
