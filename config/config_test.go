package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/config"
	"github.com/go-playground/validator/v10"
)

type DatabaseConfig struct {
	Host     string `env:"DB_HOST" envDefault:"localhost"`
	Port     int    `env:"DB_PORT" envDefault:"5432"`
	User     string `env:"DB_USER" validate:"required"`
	Password string `env:"DB_PASSWORD" validate:"required"`
}

type AppConfig struct {
	Port         int            `env:"PORT" envDefault:"8080"`
	Debug        bool           `env:"DEBUG" envDefault:"false"`
	Timeout      time.Duration  `env:"TIMEOUT" envDefault:"30s"`
	AllowedHosts []string       `env:"ALLOWED_HOSTS" envDefault:"localhost"`
	DB           DatabaseConfig
}

func TestLoad_Success(t *testing.T) {
	os.Setenv("DB_USER", "admin")
	os.Setenv("DB_PASSWORD", "secret")
	os.Setenv("PORT", "9090")
	os.Setenv("DEBUG", "true")
	os.Setenv("TIMEOUT", "60s")
	os.Setenv("ALLOWED_HOSTS", "localhost,example.com")
	defer os.Clearenv()

	var cfg AppConfig
	if err := config.Load(&cfg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("expected Port 9090, got %d", cfg.Port)
	}
	if !cfg.Debug {
		t.Errorf("expected Debug true, got false")
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected Timeout 60s, got %v", cfg.Timeout)
	}
	if len(cfg.AllowedHosts) != 2 || cfg.AllowedHosts[0] != "localhost" || cfg.AllowedHosts[1] != "example.com" {
		t.Errorf("expected AllowedHosts [localhost example.com], got %v", cfg.AllowedHosts)
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("expected DB.Host localhost, got %s", cfg.DB.Host)
	}
	if cfg.DB.Port != 5432 {
		t.Errorf("expected DB.Port 5432, got %d", cfg.DB.Port)
	}
	if cfg.DB.User != "admin" {
		t.Errorf("expected DB.User admin, got %s", cfg.DB.User)
	}
	if cfg.DB.Password != "secret" {
		t.Errorf("expected DB.Password secret, got %s", cfg.DB.Password)
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Setenv("DB_USER", "admin")
	os.Setenv("DB_PASSWORD", "secret")
	defer os.Clearenv()

	var cfg AppConfig
	if err := config.Load(&cfg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected Port 8080, got %d", cfg.Port)
	}
	if cfg.Debug {
		t.Errorf("expected Debug false, got true")
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", cfg.Timeout)
	}
	if len(cfg.AllowedHosts) != 1 || cfg.AllowedHosts[0] != "localhost" {
		t.Errorf("expected AllowedHosts [localhost], got %v", cfg.AllowedHosts)
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("expected DB.Host localhost, got %s", cfg.DB.Host)
	}
	if cfg.DB.Port != 5432 {
		t.Errorf("expected DB.Port 5432, got %d", cfg.DB.Port)
	}
}

func TestLoad_ValidationFails(t *testing.T) {
	os.Clearenv() // Missing DB_USER and DB_PASSWORD

	var cfg AppConfig
	err := config.Load(&cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify the error is of type validator.ValidationErrors
	_, ok := err.(validator.ValidationErrors)
	if !ok {
		t.Fatalf("expected validator.ValidationErrors, got %T: %v", err, err)
	}
}

func TestLoad_InvalidType(t *testing.T) {
	var cfg AppConfig
	err := config.Load(cfg) // Passing by value instead of pointer
	if err == nil || err.Error() != "cfg must be a non-nil pointer to a struct" {
		t.Errorf("expected 'cfg must be a non-nil pointer to a struct', got %v", err)
	}

	var str string
	err = config.Load(&str)
	if err == nil || err.Error() != "cfg must be a pointer to a struct" {
		t.Errorf("expected 'cfg must be a pointer to a struct', got %v", err)
	}
}

func TestLoad_NestedPtr(t *testing.T) {
	type PtrConfig struct {
		DB *DatabaseConfig
	}

	os.Setenv("DB_USER", "admin")
	os.Setenv("DB_PASSWORD", "secret")
	defer os.Clearenv()

	var cfg PtrConfig
	if err := config.Load(&cfg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.DB == nil {
		t.Fatalf("expected DB pointer to be populated, got nil")
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("expected DB.Host localhost, got %s", cfg.DB.Host)
	}
	if cfg.DB.User != "admin" {
		t.Errorf("expected DB.User admin, got %s", cfg.DB.User)
	}
}
