package migrations

import "github.com/go-gormigrate/gormigrate/v2"

func All() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		backfillDeployChap(),
		createNotifications(),
	}
}
