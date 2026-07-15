package migrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMigrationsSorted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "002_second.sql"), []byte("select 2;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001_first.sql"), []byte("select 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := Load(dir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(files) != 2 || files[0].Version != "001" || files[1].Version != "002" {
		t.Fatalf("unexpected order: %+v", files)
	}
}
