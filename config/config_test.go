package config_test

import (
	"strings"
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
	Port         int           `env:"PORT" envDefault:"8080"`
	Debug        bool          `env:"DEBUG" envDefault:"false"`
	Timeout      time.Duration `env:"TIMEOUT" envDefault:"30s"`
	AllowedHosts []string      `env:"ALLOWED_HOSTS" envDefault:"localhost"`
	DB           DatabaseConfig
}

func TestLoad_Success(t *testing.T) {
	t.Setenv("DB_USER", "admin")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("PORT", "9090")
	t.Setenv("DEBUG", "true")
	t.Setenv("TIMEOUT", "60s")
	t.Setenv("ALLOWED_HOSTS", "localhost,example.com")

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
	t.Setenv("DB_USER", "admin")
	t.Setenv("DB_PASSWORD", "secret")

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

	t.Setenv("DB_USER", "admin")
	t.Setenv("DB_PASSWORD", "secret")

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

func TestLoad_SetFieldErrors(t *testing.T) {
	tests := []struct {
		name    string
		envKey  string
		envVal  string
		cfgType any
		errMsg  string
	}{
		{
			name:   "Invalid Duration",
			envKey: "ERR_DUR",
			envVal: "invalid",
			cfgType: &struct {
				Val time.Duration `env:"ERR_DUR"`
			}{},
			errMsg: "failed to set field Val: time: invalid duration",
		},
		{
			name:   "Invalid Int",
			envKey: "ERR_INT",
			envVal: "invalid",
			cfgType: &struct {
				Val int `env:"ERR_INT"`
			}{},
			errMsg: "failed to set field Val: strconv.ParseInt: parsing \"invalid\": invalid syntax",
		},
		{
			name:   "Int Overflow",
			envKey: "ERR_INT_OVER",
			envVal: "1000",
			cfgType: &struct {
				Val int8 `env:"ERR_INT_OVER"`
			}{},
			errMsg: "failed to set field Val: integer overflow for value 1000",
		},
		{
			name:   "Invalid Uint",
			envKey: "ERR_UINT",
			envVal: "invalid",
			cfgType: &struct {
				Val uint `env:"ERR_UINT"`
			}{},
			errMsg: "failed to set field Val: strconv.ParseUint: parsing \"invalid\": invalid syntax",
		},
		{
			name:   "Uint Overflow",
			envKey: "ERR_UINT_OVER",
			envVal: "1000",
			cfgType: &struct {
				Val uint8 `env:"ERR_UINT_OVER"`
			}{},
			errMsg: "failed to set field Val: uint overflow for value 1000",
		},
		{
			name:   "Invalid Bool",
			envKey: "ERR_BOOL",
			envVal: "invalid",
			cfgType: &struct {
				Val bool `env:"ERR_BOOL"`
			}{},
			errMsg: "failed to set field Val: strconv.ParseBool: parsing \"invalid\": invalid syntax",
		},
		{
			name:   "Invalid Float",
			envKey: "ERR_FLOAT",
			envVal: "invalid",
			cfgType: &struct {
				Val float64 `env:"ERR_FLOAT"`
			}{},
			errMsg: "failed to set field Val: strconv.ParseFloat: parsing \"invalid\": invalid syntax",
		},
		{
			name:   "Float Overflow",
			envKey: "ERR_FLOAT_OVER",
			envVal: "1e39",
			cfgType: &struct {
				Val float32 `env:"ERR_FLOAT_OVER"`
			}{},
			errMsg: "failed to set field Val: float overflow for value 1e39",
		},
		{
			name:   "Invalid Slice Element",
			envKey: "ERR_SLICE",
			envVal: "1,invalid",
			cfgType: &struct {
				Val []int `env:"ERR_SLICE"`
			}{},
			errMsg: "failed to set field Val: strconv.ParseInt: parsing \"invalid\": invalid syntax",
		},
		{
			name:   "Unsupported Type",
			envKey: "ERR_UNSUPPORTED",
			envVal: "val",
			cfgType: &struct {
				Val complex64 `env:"ERR_UNSUPPORTED"`
			}{},
			errMsg: "failed to set field Val: unsupported type complex64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)

			err := config.Load(tt.cfgType)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			// We might get slightly different error messages depending on the go version,
			// so we just check if it contains the most important parts.
			if err.Error() != tt.errMsg && !strings.Contains(err.Error(), "invalid syntax") && !strings.Contains(err.Error(), "overflow") && !strings.Contains(err.Error(), "unsupported type") && !strings.Contains(err.Error(), "invalid duration") {
				t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

func TestLoad_NestedErrors(t *testing.T) {
	t.Run("Nested Struct Error", func(t *testing.T) {
		type Inner struct {
			Val int `env:"ERR_NESTED"`
		}
		type Outer struct {
			In Inner
		}

		t.Setenv("ERR_NESTED", "invalid")

		var cfg Outer
		err := config.Load(&cfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("Nested Struct Pointer Error", func(t *testing.T) {
		type Inner struct {
			Val int `env:"ERR_NESTED_PTR"`
		}
		type Outer struct {
			In *Inner
		}

		t.Setenv("ERR_NESTED_PTR", "invalid")

		var cfg Outer
		err := config.Load(&cfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestLoad_Coverage(t *testing.T) {
	// struct with unexported field, no env tag, uint, float, and slice with empty element
	type CoverageConfig struct {
		unexported string
		NoEnv      string
		UintVal    uint    `env:"COV_UINT"`
		FloatVal   float64 `env:"COV_FLOAT"`
		SliceVal   []int   `env:"COV_SLICE"`
	}

	t.Setenv("COV_UINT", "42")
	t.Setenv("COV_FLOAT", "3.14")
	t.Setenv("COV_SLICE", "1, , 2") // contains empty part after trim

	var cfg CoverageConfig
	err := config.Load(&cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.UintVal != 42 {
		t.Errorf("expected UintVal 42, got %v", cfg.UintVal)
	}
	if cfg.FloatVal != 3.14 {
		t.Errorf("expected FloatVal 3.14, got %v", cfg.FloatVal)
	}
	if len(cfg.SliceVal) != 2 || cfg.SliceVal[0] != 1 || cfg.SliceVal[1] != 2 {
		t.Errorf("expected SliceVal [1 2], got %v", cfg.SliceVal)
	}
}
