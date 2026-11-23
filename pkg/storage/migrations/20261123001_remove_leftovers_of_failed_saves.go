package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// removeLeftoversOfFailedSaves deletes the rows failed saves left behind, each of which holds a
// name against later saves: databases without a file, and file stores no database points at once
// relinkFileStores has run. A link to a file store that no longer exists is reset, since it fails
// every deploy from that database. Nothing is saving while migrations run, so a database without a
// file is never one whose dump is still in progress.
func removeLeftoversOfFailedSaves() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20261123001",
		Migrate: func(tx *gorm.DB) error {
			statements := []string{
				`DELETE FROM databases WHERE type IS DISTINCT FROM 'fs' AND COALESCE(url, '') = ''`,
				`UPDATE databases SET filestore_id = 0
WHERE COALESCE(filestore_id, 0) <> 0
	AND NOT EXISTS (SELECT 1 FROM databases AS file_store WHERE file_store.id = databases.filestore_id)`,
				`DELETE FROM databases AS file_store
WHERE file_store.type = 'fs'
	AND NOT EXISTS (SELECT 1 FROM databases AS linked WHERE linked.filestore_id = file_store.id)`,
			}
			for _, statement := range statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
