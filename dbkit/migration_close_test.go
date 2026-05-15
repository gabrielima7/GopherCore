package dbkit

import (
	"errors"
	"os"
	"path/filepath"
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
	// Windows absolute paths might have a colon, but url parsing in migrate
	// might treat it differently. file source driver uses the raw string usually,
	// so let's unprefix errclosefile:// and open that.
	d, err := f.Open("file://" + strings.TrimPrefix(url, "errclosefile://"))
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

	// Ensure all backslashes are replaced by forward slashes to correctly format as file URL
	path = strings.ReplaceAll(path, "\\", "/")

	// Convert to file:// format properly. For windows this is file:///C:/path
	// This is because the url parser treats the first part as host if not 3 slashes.
	if filepath.IsAbs(path) && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return "file://" + path, "errclosefile://" + path
}

func TestRunMigrations_CloseErrors(t *testing.T) {
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
