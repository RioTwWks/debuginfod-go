package storage

import "database/sql"

func migrateUIIndexes(db *sql.DB) error {
	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_artifacts_git_commit ON artifacts(git_commit)",
		"CREATE INDEX IF NOT EXISTS idx_artifacts_file_path ON artifacts(file_path)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
