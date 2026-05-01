// Package config provides a unified, reflection-based configuration loader.
// It parses environment variables directly into structured Go types and enforces
// validation constraints via the github.com/go-playground/validator/v10 library,
// ensuring that the application fails to start if vital configurations are missing
// or malformed.
package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// Load reflects upon the provided cfg parameter, recursively parsing environment
// variables into its exported fields. It then validates the populated struct against
// its `validate` tags using the go-playground/validator library.
//
// Purpose: Automatically loads and validates configuration data directly from the environment.
// Constraints: The cfg parameter MUST be a non-nil pointer to a struct. It returns an error if
// reflection checks fail, if parsing/casting a value fails, or if validation rules are violated.
// Thread-safety: Load relies on a global, thread-safe validator instance. Safe for concurrent use,
// though normally invoked once at application startup.
//
// Tag Usage:
//   - `env:"NAME"`: Binds the struct field to the environment variable NAME.
//   - `envDefault:"val"`: Uses "val" if the specified environment variable is absent or empty.
//   - `validate:"rule"`: Applies standard go-playground validation rules.
func Load(cfg any) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return errors.New("cfg must be a non-nil pointer to a struct")
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return errors.New("cfg must be a pointer to a struct")
	}

	if err := populate(v); err != nil {
		return err
	}

	return validate.Struct(cfg)
}

// populate iterates over the fields of the reflected struct, recursively
// diving into nested structs and pointers to structs. It extracts values
// from the environment and attempts to parse and set them dynamically.
//
// Purpose: This is the core logic that connects `env` string tags to actual OS
// environment queries, abstracting the manual `os.LookupEnv` boilerplate.
// Thread-safety: Safe for concurrent use so long as the target struct is not accessed.
func populate(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Ignore unexported or otherwise unsettable fields.
		if !fieldValue.CanSet() {
			continue
		}

		// Handle nested struct values recursively.
		if fieldValue.Kind() == reflect.Struct {
			if err := populate(fieldValue); err != nil {
				return err
			}
			continue
		}

		// Handle nested pointers to structs recursively, allocating if nil.
		if fieldValue.Kind() == reflect.Pointer && fieldValue.Type().Elem().Kind() == reflect.Struct {
			if fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
			}
			if err := populate(fieldValue.Elem()); err != nil {
				return err
			}
			continue
		}

		// Skip fields that do not have an explicit `env` tag mapping.
		envKey := field.Tag.Get("env")
		if envKey == "" {
			continue
		}

		envVal, exists := os.LookupEnv(envKey)
		if !exists {
			defaultVal := field.Tag.Get("envDefault")
			if defaultVal == "" {
				continue
			}
			envVal = defaultVal
		}

		if err := setField(fieldValue, envVal); err != nil {
			return fmt.Errorf("failed to set field %s: %w", field.Name, err)
		}
	}
	return nil
}

// setField parses the string value obtained from the environment and assigns it
// to the reflected target value, strictly checking for numeric overflows to prevent
// silent truncation bugs at startup.
//
// Purpose: Handles type conversions from env string slices, floats, booleans, and duration types.
// Thread-safety: Safe for concurrent use.
func setField(v reflect.Value, value string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			v.SetInt(int64(d))
			return nil
		}

		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		if v.OverflowInt(intVal) {
			return fmt.Errorf("integer overflow for value %s", value)
		}
		v.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		if v.OverflowUint(uintVal) {
			return fmt.Errorf("uint overflow for value %s", value)
		}
		v.SetUint(uintVal)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		v.SetBool(boolVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		if v.OverflowFloat(floatVal) {
			return fmt.Errorf("float overflow for value %s", value)
		}
		v.SetFloat(floatVal)
	case reflect.Slice:
		parts := strings.Split(value, ",")
		slice := reflect.MakeSlice(v.Type(), 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			elem := reflect.New(v.Type().Elem()).Elem()
			if err := setField(elem, part); err != nil {
				return err
			}
			slice = reflect.Append(slice, elem)
		}
		v.Set(slice)
	default:
		return fmt.Errorf("unsupported type %s", v.Kind())
	}
	return nil
}
