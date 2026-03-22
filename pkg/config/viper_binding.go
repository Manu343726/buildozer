package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

// bindStructToViper uses reflection to automatically bind all struct fields to viper environment variables.
// It walks through the Config struct hierarchy and binds each field to the corresponding
// environment variable using the struct tags (yaml, json) as hints for key naming.
func bindStructToViper(prefix string, v reflect.Value, t reflect.Type) error {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	// Only process structs
	if t.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get the viper key name from struct tags (prefer yaml, then json, then field name)
		keyName := getKeyName(field)
		fullKey := keyName
		if prefix != "" {
			fullKey = prefix + "." + keyName
		}

		// Handle nested structs recursively
		if field.Type.Kind() == reflect.Struct {
			if err := bindStructToViper(fullKey, fieldValue, field.Type); err != nil {
				return err
			}
			continue
		}

		// Handle pointer to struct
		if field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct {
			if !fieldValue.IsNil() {
				if err := bindStructToViper(fullKey, fieldValue, field.Type); err != nil {
					return err
				}
			}
			continue
		}

		// For scalar types, bind environment variable
		envVarName := makeEnvVarName(fullKey)
		viper.BindEnv(fullKey, envVarName)
	}

	return nil
}

// populateStructFromViper uses reflection to extract values from viper and populate
// a struct. It walks through the struct hierarchy and uses viper keys derived from
// struct tags to fetch values.
func populateStructFromViper(prefix string, v reflect.Value, t reflect.Type) {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	// Only process structs
	if t.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get the viper key name from struct tags
		keyName := getKeyName(field)
		fullKey := keyName
		if prefix != "" {
			fullKey = prefix + "." + keyName
		}

		// Handle nested structs recursively
		if field.Type.Kind() == reflect.Struct {
			populateStructFromViper(fullKey, fieldValue, field.Type)
			continue
		}

		// Handle pointer to struct
		if field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct {
			if !fieldValue.IsNil() {
				populateStructFromViper(fullKey, fieldValue, field.Type)
			}
			continue
		}

		// Get value from viper and set it in the struct
		if viper.IsSet(fullKey) {
			setFieldFromViper(fieldValue, field.Type, fullKey)
		}
	}
}

// setFieldFromViper sets a struct field value from viper based on the field type
func setFieldFromViper(fieldValue reflect.Value, fieldType reflect.Type, viperKey string) {
	switch fieldType.Kind() {
	case reflect.String:
		fieldValue.SetString(viper.GetString(viperKey))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fieldValue.SetInt(int64(viper.GetInt(viperKey)))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fieldValue.SetUint(uint64(viper.GetUint(viperKey)))
	case reflect.Bool:
		fieldValue.SetBool(viper.GetBool(viperKey))
	case reflect.Float32, reflect.Float64:
		fieldValue.SetFloat(viper.GetFloat64(viperKey))
	}
}

// getKeyName extracts the viper key name from struct field tags.
// Prefers yaml tag, then json tag, falls back to field name (lowercased).
func getKeyName(field reflect.StructField) string {
	// Try yaml tag first
	if yamlTag := field.Tag.Get("yaml"); yamlTag != "" {
		// Handle "field,omitempty" format
		parts := strings.Split(yamlTag, ",")
		if parts[0] != "-" && parts[0] != "" {
			return parts[0]
		}
	}

	// Try json tag
	if jsonTag := field.Tag.Get("json"); jsonTag != "" {
		// Handle "field,omitempty" format
		parts := strings.Split(jsonTag, ",")
		if parts[0] != "-" && parts[0] != "" {
			return parts[0]
		}
	}

	// Default: use lowercased field name
	return strings.ToLower(field.Name)
}

// makeEnvVarName converts a viper key (e.g., "daemon.host") to an environment variable name
// (e.g., "BUILDOZER_DAEMON_HOST").
func makeEnvVarName(viperKey string) string {
	parts := strings.Split(viperKey, ".")
	for i, part := range parts {
		parts[i] = strings.ToUpper(part)
	}
	return "BUILDOZER_" + strings.Join(parts, "_")
}

// BindConfigToViper automatically binds all fields of the Config struct to viper
// using reflection and environment variable injection.
func BindConfigToViper(cfg *Config) error {
	viper.SetEnvPrefix("BUILDOZER")

	// Use reflection to walk through the Config struct and bind all fields
	if err := bindStructToViper("", reflect.ValueOf(cfg), reflect.TypeOf(cfg)); err != nil {
		return fmt.Errorf("failed to bind config to viper: %w", err)
	}

	return nil
}
