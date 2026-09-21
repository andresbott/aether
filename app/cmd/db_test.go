package cmd

import (
	"os"
	"path/filepath"
	"testing"

	gormlogger "gorm.io/gorm/logger"
)

func TestOpenDB(t *testing.T) {
	dataDir := t.TempDir()
	db, err := openDB(dataDir, gormlogger.Discard)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	var mode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}

	// A write makes SQLite create its -wal/-shm files, which it keeps next to
	// the database: they must land in the sqlite directory too.
	if err := db.Exec("CREATE TABLE probe (x INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "sqlite", "aether.db")); err != nil {
		t.Errorf("database not created under DataDir/sqlite: %v", err)
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != "sqlite" {
		t.Errorf("DataDir root holds %v, want only the sqlite directory", names)
	}
}
