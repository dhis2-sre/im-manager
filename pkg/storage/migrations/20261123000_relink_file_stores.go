package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// relinkFileStores points databases back at the file store they lost. Saving a database used to
// reset its filestore_id to 0 while the file store row and its tarball stayed, so the next save
// failed on the row's name as "already exists" and seeds from the database restored no files. A
// file store is named after its database, the extension replaced by "-fs.tar.gz", which is how the
// two are matched here. A file store is only given to a database when no database points at it and
// no second database of the group maps to its name, since deleting a database deletes its file
// store.
func relinkFileStores() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20261123000",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec(`WITH expected AS (
	SELECT id, group_name,
		regexp_replace(regexp_replace(regexp_replace(name, '\.sql\.gz$', ''), '\.pgc$', ''), '\.tar\.gz$', '') || '-fs.tar.gz' AS file_store_name
	FROM databases
	WHERE type IS DISTINCT FROM 'fs'
), unambiguous AS (
	SELECT min(id) AS id, group_name, file_store_name
	FROM expected
	GROUP BY group_name, file_store_name
	HAVING count(*) = 1
)
UPDATE databases
SET filestore_id = file_store.id
FROM unambiguous
JOIN databases AS file_store ON file_store.type = 'fs' AND file_store.group_name = unambiguous.group_name AND file_store.name = unambiguous.file_store_name
WHERE databases.id = unambiguous.id
	AND COALESCE(databases.filestore_id, 0) = 0
	AND NOT EXISTS (SELECT 1 FROM databases AS linked WHERE linked.filestore_id = file_store.id)`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
