// Package dbkit provides thread-safe database connection management, robust connection pooling defaults,
// and safe schema migration orchestration built upon sqlx and golang-migrate/migrate.
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
//
// Purpose: Dictates source targets for schema application.
// Constraints: None.
// Errors: None.
// Thread-safety: Read-only after instantiation.
type MigrationConfig struct {
	// SourceURL is the source URL for migration files (e.g., "file://migrations").
	SourceURL string
	// DatabaseName is the database driver name for migrate (e.g., "postgres").
	DatabaseName string
}

// RunMigrations incrementally applies all pending "up" migrations located at the specified sourceURL.
//
// Purpose: Relies on golang-migrate to orchestrate the internal schema_migrations table safely.
// Constraints: Note that schema migrations often perform DDL operations that cannot be fully encapsulated in a transaction depending on the underlying database engine. Ensure backups are available.
// Errors: Bubbles up any failure during execution.
// Thread-safety: Operations are inherently stateful on the database side; concurrent migration execution from multiple nodes is usually handled safely by golang-migrate's internal advisory locks.
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

// RollbackMigrations selectively reverts the last N migration steps by executing their
// corresponding "down" migration files. If the steps parameter is exactly 0, it will
// systematically revert all previously applied migrations.
//
// Purpose: Allows manual unspooling of failed or errant database schemas.
// Constraints: Like RunMigrations, destructive DDL side-effects may occur and not all databases support rolling back these types of operations transactionally.
// Errors: Bubbles up execution errors.
// Thread-safety: Concurrent execution relies on the underlying golang-migrate advisory locks on the DB.
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
//
// Purpose: Describes the currently applied schema sequence.
// Constraints: None.
// Errors: None.
// Thread-safety: Pure struct, inherently safe.
type MigrationVersion struct {
	Version uint
	Dirty   bool
}

// GetMigrationVersion queries the underlying migrate state machine to retrieve the current
// active schema version.
//
// Purpose: Safely inspects DB state machine.
// Constraints: It also returns a "dirty" boolean flag, which if true, indicates that the last attempted migration failed midway, leaving the database in a potentially inconsistent state.
// Errors: Bubbles up connection or parse errors.
// Thread-safety: Safe for concurrent queries across multiple nodes reading state.
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
