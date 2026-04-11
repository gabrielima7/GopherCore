package dbkit

import (
	"errors"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	// Source driver for file-based migrations.
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
)

// MigrationConfig holds configuration for running migrations.
type MigrationConfig struct {
	// SourceURL is the source URL for migration files (e.g., "file://migrations").
	SourceURL string
	// DatabaseName is the database driver name for migrate (e.g., "postgres").
	DatabaseName string
}

// RunMigrations applies all pending migrations.
// The migrationsPath should be a URL like "file://./migrations".
func RunMigrations(db *sqlx.DB, driverName string, driver database.Driver, sourceURL string) error {
	m, err := migrate.NewWithDatabaseInstance(sourceURL, driverName, driver)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// RollbackMigrations rolls back N migration steps.
// If steps is 0, it rolls back all migrations.
func RollbackMigrations(db *sqlx.DB, driverName string, driver database.Driver, sourceURL string, steps int) error {
	m, err := migrate.NewWithDatabaseInstance(sourceURL, driverName, driver)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = m.Close()
	}()

	if steps <= 0 {
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		return nil
	}

	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrationVersion represents the current migration state.
type MigrationVersion struct {
	Version uint
	Dirty   bool
}

// GetMigrationVersion returns the current migration version and dirty flag.
func GetMigrationVersion(driverName string, driver database.Driver, sourceURL string) (MigrationVersion, error) {
	m, err := migrate.NewWithDatabaseInstance(sourceURL, driverName, driver)
	if err != nil {
		return MigrationVersion{}, err
	}
	defer func() {
		_, _ = m.Close()
	}()

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return MigrationVersion{Version: 0, Dirty: false}, nil
		}
		return MigrationVersion{}, err
	}

	return MigrationVersion{Version: version, Dirty: dirty}, nil
}
