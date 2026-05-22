// Package dbkit provides database operations, including schema migration wrappers.
// Purpose: Automates safe database schema migrations.
// Constraints: Relies heavily on golang-migrate/migrate.
// Thread-safety: Functions are safe for concurrent use.
package dbkit

import (
	"errors"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	// Source driver for file-based migrations.
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
)

// MigrationConfig structures the mandatory routing parameters required to interface the schema migrator with physical filesystem directories and logical database drivers.
// Purpose: Bundles migration source and database driver configs.
// Constraints: Must point to a valid driver and source URL.
// Thread-safety: Read-only after instantiation.
type MigrationConfig struct {
	// SourceURL is the source URL for migration files (e.g., "file://migrations").
	// Purpose: Identifies where the migration files are located.
	// Constraints: Must be a valid URL scheme supported by golang-migrate.
	// Thread-safety: Read-only string.
	SourceURL string
	// DatabaseName is the database driver name for migrate (e.g., "postgres").
	// Purpose: Tells the migrator which SQL dialect to use.
	// Constraints: Must match the registered database driver name.
	// Thread-safety: Read-only string.
	DatabaseName string
}

// RunMigrations commands a forward-moving schema reconciliation, scanning the specified source directory for pending DDL patches and applying them iteratively to the active database connection.
// Purpose: Automates schema upgrades against the connected database.
// Constraints: Note that schema migrations often perform DDL operations that cannot be fully encapsulated in
// a transaction depending on the underlying database engine. Ensure backups are available.
// Thread-safety: Operations are inherently stateful on the database side; concurrent migration execution from
// multiple nodes is usually handled safely by golang-migrate's internal advisory locks.
func RunMigrations(db *sqlx.DB, driverName string, driver database.Driver, sourceURL string) (err error) {
	m, err := migrate.NewWithDatabaseInstance(sourceURL, driverName, driver)
	if err != nil {
		return err
	}
	defer func() {
		// Ensures the migration engine drops its internal connections and advisory locks
		// cleanly, regardless of whether the migration succeeded or failed.
		sourceErr, dbErr := m.Close()
		if sourceErr != nil && err == nil {
			err = sourceErr
		}
		if dbErr != nil && err == nil {
			err = dbErr
		}
	}()

	// ErrNoChange is explicitly ignored because reaching the target version successfully
	// without applying new steps is considered a valid, non-erroneous terminal state.
	if upErr := m.Up(); upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return upErr
	}
	return nil
}

// RollbackMigrations forces a backwards schema degradation, unrolling the specified number of historical database patches by executing their corresponding destructive 'down' DDL instructions.
// Purpose: Reverts applied database schema migrations.
// Constraints: Like RunMigrations, destructive DDL side-effects may occur and not all databases support
// rolling back these types of operations transactionally.
// Thread-safety: Concurrent execution relies on the underlying golang-migrate advisory locks on the DB.
func RollbackMigrations(db *sqlx.DB, driverName string, driver database.Driver, sourceURL string, steps int) (err error) {
	m, err := migrate.NewWithDatabaseInstance(sourceURL, driverName, driver)
	if err != nil {
		return err
	}
	defer func() {
		sourceErr, dbErr := m.Close()
		if sourceErr != nil && err == nil {
			err = sourceErr
		}
		if dbErr != nil && err == nil {
			err = dbErr
		}
	}()

	// A value of 0 or below signals a total teardown, dropping all schema versions dynamically.
	if steps <= 0 {
		if downErr := m.Down(); downErr != nil && !errors.Is(downErr, migrate.ErrNoChange) {
			return downErr
		}
		return nil
	}

	// Step backwards exactly N times. The negative integer signifies the inverse direction.
	if stepErr := m.Steps(-steps); stepErr != nil && !errors.Is(stepErr, migrate.ErrNoChange) {
		return stepErr
	}
	return nil
}

// MigrationVersion captures the temporal snapshot of the database's structural integrity, signaling both the active schema integer and whether a previous rollout crashed mid-flight.
// Purpose: Models the version and dirty state flag.
// Constraints: A dirty flag typically blocks further migrations until manually resolved.
// Thread-safety: Struct data.
type MigrationVersion struct {
	// Version is the current numeric version of the database schema.
	// Purpose: Represents the currently applied migration step.
	// Constraints: Usually an incrementally increasing integer.
	// Thread-safety: Read-only primitive.
	Version uint
	// Dirty indicates whether the last migration attempt failed and left the schema in a potentially inconsistent state.
	// Purpose: Flags if the database requires manual intervention to fix a botched migration.
	// Constraints: If true, automated migrations will typically halt.
	// Thread-safety: Read-only boolean.
	Dirty bool
}

// GetMigrationVersion queries the internal synchronization tables inside the target database to extract the currently recognized schema generation timestamp alongside its cleanliness flag.
// Purpose: Reads the active database schema version level.
// Constraints: It also returns a "dirty" boolean flag, which if true, indicates that
// the last attempted migration failed midway, leaving the database in a potentially inconsistent state.
// Thread-safety: Safe for concurrent queries across multiple nodes reading state.
func GetMigrationVersion(driverName string, driver database.Driver, sourceURL string) (mv MigrationVersion, err error) {
	m, err := migrate.NewWithDatabaseInstance(sourceURL, driverName, driver)
	if err != nil {
		return MigrationVersion{}, err
	}
	defer func() {
		sourceErr, dbErr := m.Close()
		if sourceErr != nil && err == nil {
			err = sourceErr
		}
		if dbErr != nil && err == nil {
			err = dbErr
		}
	}()

	version, dirty, verErr := m.Version()
	if verErr != nil {
		// migrate.ErrNilVersion indicates no migrations have been applied yet.
		// We safely absorb this specific error and report version 0.
		if errors.Is(verErr, migrate.ErrNilVersion) {
			return MigrationVersion{Version: 0, Dirty: false}, nil
		}
		return MigrationVersion{}, verErr
	}

	return MigrationVersion{Version: version, Dirty: dirty}, nil
}
