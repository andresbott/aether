package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// The SQLite database lives in a directory of its own under DataDir, so the
// -wal and -shm files SQLite keeps next to it stay out of DataDir's root.
const (
	sqliteDir = "sqlite"
	dbFile    = "aether.db"
)

// dbPath returns where the SQLite database lives under dataDir.
func dbPath(dataDir string) string {
	return filepath.Join(dataDir, sqliteDir, dbFile)
}

// openDB opens the SQLite database under dataDir, creating it — and its
// directory — when missing.
func openDB(dataDir string, gormLog gormlogger.Interface) (*gorm.DB, error) {
	path := dbPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}
	// busy_timeout is per-connection state, so it must be set in the DSN: a PRAGMA
	// issued via db.Exec runs on a single pooled connection and leaves the other
	// nine at the default of 0, which returns SQLITE_BUSY immediately under write
	// contention instead of waiting. journal_mode=WAL is recorded in the database
	// file, so the one-off Exec below suffices for it.
	db, err := gorm.Open(sqlite.Open(path+"?_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: gormLog,
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	db.Exec("PRAGMA journal_mode=WAL")
	return db, nil
}
