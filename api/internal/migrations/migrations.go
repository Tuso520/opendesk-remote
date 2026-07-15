package migrations

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	Version string
	Path    string
	SQL     string
}

func Load(dir string) ([]File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, _, ok := strings.Cut(entry.Name(), "_")
		if !ok || version == "" {
			return nil, errors.New("migration filename must start with numeric version and underscore: " + entry.Name())
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Version: version, Path: path, SQL: string(raw)})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Version < files[j].Version
	})
	return files, nil
}

func Apply(ctx context.Context, db *sql.DB, dir string) error {
	files, err := Load(dir)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(64) PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	for _, file := range files {
		var version string
		err := db.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version = ?`, file.Version).Scan(&version)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := db.ExecContext(ctx, file.SQL); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES(?)`, file.Version); err != nil {
			return err
		}
	}
	return nil
}
