package migrations

import "github.com/go-gormigrate/gormigrate/v2"

func All() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		backfillDeployChap(),
		reencryptCFBToGCM(),
		backfillDeployChapDHIS2V2(),
		backfillDeployStatus(),
		decryptFilesystemVolumeSize(),
		presetNames(),
		raiseDatabaseMemoryRequest(),
		dropLegacyDeploymentNameIndex(),
		backfillSuspendAndGlowrootDHIS2V2(),
		raiseDHIS2V2ChartVersion(),
	}
}
