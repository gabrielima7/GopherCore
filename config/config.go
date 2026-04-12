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

// Load parses environment variables into a struct and validates it using go-playground/validator/v10.
// It uses `env` to lookup environment variables and `envDefault` for default values.
// `validate` tag is used to enforce validation rules.
func Load(cfg any) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.IsNil() {
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

func populate(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		if fieldValue.Kind() == reflect.Struct {
			if err := populate(fieldValue); err != nil {
				return err
			}
			continue
		}

		if fieldValue.Kind() == reflect.Ptr && fieldValue.Type().Elem().Kind() == reflect.Struct {
			if fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
			}
			if err := populate(fieldValue.Elem()); err != nil {
				return err
			}
			continue
		}

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
		v.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
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
		v.SetFloat(floatVal)
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.String {
			parts := strings.Split(value, ",")
			slice := reflect.MakeSlice(v.Type(), len(parts), len(parts))
			for i, part := range parts {
				slice.Index(i).SetString(strings.TrimSpace(part))
			}
			v.Set(slice)
			return nil
		}
		return fmt.Errorf("unsupported slice type %s", v.Type().Elem().Kind())
	default:
		return fmt.Errorf("unsupported type %s", v.Kind())
	}
	return nil
}
