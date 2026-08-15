package dbkit

import (
	"errors"
	"go.uber.org/goleak"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
)

type errCloseDriver struct {
	database.Driver
	closeErr error
}

func (d *errCloseDriver) Close() error {
	_ = d.Driver.Close()
	return d.closeErr
}

type errCloseSource struct {
	source.Driver
}

func (s *errCloseSource) Close() error {
	_ = s.Driver.Close()
	return errors.New("simulated source close error")
}

func (s *errCloseSource) Open(url string) (source.Driver, error) {
	f := &file.File{}

	// Unprefix our custom scheme
	pathStr := strings.TrimPrefix(url, "errclosefile://")

	// Open it using the standard file source driver mechanism
	d, err := f.Open("file://" + pathStr)
	if err != nil {
		return nil, err
	}
	return &errCloseSource{Driver: d}, nil
}

func init() {
	source.Register("errclosefile", &errCloseSource{})
}

func getSourceURLs(t *testing.T) (string, string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	path := filepath.Join(wd, "testdata", "migrations")

	if _, err := os.Stat(path); err != nil {
		path = filepath.Join("testdata", "migrations")
	}

	path = filepath.ToSlash(path)

	// In migrate/v4 the file scheme expects the path string.
	// We want to pass an absolute file URL for local filesystem if possible,
	// but on Windows if we format as file:///C:/path, url.Parse sometimes trips or file.Open treats it weirdly.
	// The standard way migrate handles Windows paths is to just prefix `file://` directly to `C:/path`
	// which parses into Opaque rather than Host/Path, OR use relative paths like `file://./testdata`.
	// For robust testing across OSes, using relative paths is safest to avoid Windows drive letter colon issues.

	// Convert to relative if we are already inside the package dir
	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(wd, filepath.FromSlash(path))
		if err == nil {
			path = filepath.ToSlash(rel)
		}
	}

	if !strings.HasPrefix(path, ".") && !filepath.IsAbs(path) {
		path = "./" + path
	}

	// Just prefix file:// to whatever the path is.
	// If it's relative like file://./testdata/migrations it works perfectly on all OS.
	// If it's absolute, Windows path parsing with colon might be tricky, so relative is best.

	// However, if we absolutely must pass absolute Windows path, file:///C:/... is technically correct.
	// We'll stick to relative if possible for safety to avoid url.Parse issues in golang-migrate source/file driver.
	if filepath.IsAbs(path) && runtime.GOOS == "windows" {
		// e.g. C:/path -> file:///C:/path
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	return "file://" + path, "errclosefile://" + path
}

func TestRunMigrations_CloseErrors(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	dbPath := newTestMigrationEnv(t)
	sourceURL, errSourceURL := getSourceURLs(t)

	tests := []struct {
		name          string
		sourceURL     string
		wrapDriver    func(database.Driver) database.Driver
		expectedError string
	}{
		{
			name:      "Database close error",
			sourceURL: sourceURL,
			wrapDriver: func(d database.Driver) database.Driver {
				return &errCloseDriver{Driver: d, closeErr: errors.New("simulated db close error")}
			},
			expectedError: "simulated db close error",
		},
		{
			name:      "Source close error",
			sourceURL: errSourceURL,
			wrapDriver: func(d database.Driver) database.Driver {
				return d
			},
			expectedError: "simulated source close error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawDB := openDB(t, dbPath)
			realDriver := newDriver(t, rawDB)
			db := sqlx.NewDb(rawDB, "sqlite3")

			driver := tt.wrapDriver(realDriver)
			err := RunMigrations(db, "sqlite3", driver, tt.sourceURL)

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.expectedError {
				t.Fatalf("expected %q, got %q", tt.expectedError, err.Error())
			}
		})
	}
}

func TestRollbackMigrations_CloseErrors(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	dbPath := newTestMigrationEnv(t)
	sourceURL, errSourceURL := getSourceURLs(t)

	tests := []struct {
		name          string
		sourceURL     string
		wrapDriver    func(database.Driver) database.Driver
		expectedError string
	}{
		{
			name:      "Database close error",
			sourceURL: sourceURL,
			wrapDriver: func(d database.Driver) database.Driver {
				return &errCloseDriver{Driver: d, closeErr: errors.New("simulated db close error")}
			},
			expectedError: "simulated db close error",
		},
		{
			name:      "Source close error",
			sourceURL: errSourceURL,
			wrapDriver: func(d database.Driver) database.Driver {
				return d
			},
			expectedError: "simulated source close error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pre-apply migrations for the step to work
			rawDBInit := openDB(t, dbPath)
			initDriver := newDriver(t, rawDBInit)
			dbInit := sqlx.NewDb(rawDBInit, "sqlite3")
			err := RunMigrations(dbInit, "sqlite3", initDriver, sourceURL)
			if err != nil {
				t.Fatalf("failed to init migrations: %v", err)
			}

			rawDB := openDB(t, dbPath)
			realDriver := newDriver(t, rawDB)
			db := sqlx.NewDb(rawDB, "sqlite3")

			driver := tt.wrapDriver(realDriver)
			err = RollbackMigrations(db, "sqlite3", driver, tt.sourceURL, 1)

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.expectedError {
				t.Fatalf("expected %q, got %q", tt.expectedError, err.Error())
			}
		})
	}
}

func TestGetMigrationVersion_CloseErrors(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	dbPath := newTestMigrationEnv(t)
	sourceURL, errSourceURL := getSourceURLs(t)

	tests := []struct {
		name          string
		sourceURL     string
		wrapDriver    func(database.Driver) database.Driver
		expectedError string
	}{
		{
			name:      "Database close error",
			sourceURL: sourceURL,
			wrapDriver: func(d database.Driver) database.Driver {
				return &errCloseDriver{Driver: d, closeErr: errors.New("simulated db close error")}
			},
			expectedError: "simulated db close error",
		},
		{
			name:      "Source close error",
			sourceURL: errSourceURL,
			wrapDriver: func(d database.Driver) database.Driver {
				return d
			},
			expectedError: "simulated source close error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawDB := openDB(t, dbPath)
			realDriver := newDriver(t, rawDB)

			driver := tt.wrapDriver(realDriver)
			_, err := GetMigrationVersion("sqlite3", driver, tt.sourceURL)

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.expectedError {
				t.Fatalf("expected %q, got %q", tt.expectedError, err.Error())
			}
		})
	}
}
