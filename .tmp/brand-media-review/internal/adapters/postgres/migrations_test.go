package postgres

import (
	"io/fs"
	"testing"
)

func TestMigrationVersion(t *testing.T) {
	version, err := migrationVersion("0007_example.sql")
	if err != nil || version != 7 {
		t.Fatalf("expected version 7, got %d, err=%v", version, err)
	}
}

func TestMigrationVersionRejectsUnversionedFile(t *testing.T) {
	if _, err := migrationVersion("initial.sql"); err == nil {
		t.Fatal("expected unversioned migration to fail")
	}
}

func TestMigrationVersionsAreUnique(t *testing.T) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	seen := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			t.Fatalf("migration %s: %v", entry.Name(), err)
		}
		if previous, exists := seen[version]; exists {
			t.Fatalf("migration version %d is used by both %s and %s", version, previous, entry.Name())
		}
		seen[version] = entry.Name()
	}
}
