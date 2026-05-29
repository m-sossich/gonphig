package gonphig

import (
	"errors"
	"flag"
	"fmt"
	"github.com/m-sossich/gonphig/internal/validation"
	"gopkg.in/yaml.v3"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	readEnvKey  = "env"
	readFlagKey = "flag"
	defaultKey  = "default"
	flagUsage   = "flag-usage"
)

// ReadFromFile loads configurations into the config struct provided.
// YAML file values are the lowest priority — env vars and flags override them.
// The caller is responsible for calling fs.Parse after this returns.
func ReadFromFile(configPath string, fs *flag.FlagSet, c interface{}) error {
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(configFile, c); err != nil {
		return err
	}
	return ReadConfig(fs, c)
}

// ReadConfig loads configurations into the config struct provided.
// Pass flag.CommandLine for the standard case, or a custom *flag.FlagSet for isolation (tests, libraries).
// The caller is responsible for calling fs.Parse after this returns so registered flags are populated.
func ReadConfig(fs *flag.FlagSet, c interface{}) error {
	t := reflect.TypeOf(c)

	switch t.Kind() {
	case reflect.Ptr, reflect.Interface:
		val := t.Elem()
		for i := 0; i < val.NumField(); i++ {
			value := reflect.ValueOf(c).Elem().Field(i)
			if err := overwriteFields(fs, val.Field(i), &value); err != nil {
				return err
			}
		}
		return validation.ValidateRequired(c)
	case reflect.Struct:
		return errors.New("configuration to load needs to be a pointer")
	default:
		return errors.New("invalid configuration structure")
	}
}

var durationType = reflect.TypeOf(time.Duration(0))

func overwriteFields(fs *flag.FlagSet, f reflect.StructField, v *reflect.Value) error {
	if f.Type == durationType {
		return overwriteValue(f, v, func(v *reflect.Value, t reflect.StructTag) error { return setDuration(fs, v, t) })
	}
	switch f.Type.Kind() {
	case reflect.Struct:
		for i := 0; i < f.Type.NumField(); i++ {
			value := v.Field(i)
			if err := overwriteFields(fs, f.Type.Field(i), &value); err != nil {
				return err
			}
		}
	case reflect.Int64:
		return overwriteValue(f, v, func(v *reflect.Value, t reflect.StructTag) error { return setInt64(fs, v, t) })
	case reflect.Int:
		return overwriteValue(f, v, func(v *reflect.Value, t reflect.StructTag) error { return setInt(fs, v, t) })
	case reflect.Float32, reflect.Float64:
		return overwriteValue(f, v, func(v *reflect.Value, t reflect.StructTag) error { return setFloat64(fs, v, t) })
	case reflect.String:
		return overwriteValue(f, v, func(v *reflect.Value, t reflect.StructTag) error { return setString(fs, v, t) })
	case reflect.Bool:
		return overwriteValue(f, v, func(v *reflect.Value, t reflect.StructTag) error { return setBool(fs, v, t) })
	case reflect.Slice:
		if f.Type.Elem().Kind() != reflect.String {
			return nil
		}
		return overwriteValue(f, v, setStringSlice)
	case reflect.Map:
		return nil
	default:
		return fmt.Errorf("invalid field[%s] type[%s]", f.Name, f.Type.Name())
	}
	return nil
}

func overwriteValue(f reflect.StructField, v *reflect.Value, setValue func(*reflect.Value, reflect.StructTag) error) error {
	if len(f.Tag) > 0 {
		return setValue(v, f.Tag)
	}
	return nil
}

func setString(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.StringVar(v.Addr().Interface().(*string), val, v.String(), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if variable := os.Getenv(val); variable != "" {
			v.SetString(variable)
			return nil
		}
	}
	if def, ok := t.Lookup(defaultKey); ok && def != "" {
		v.SetString(strings.TrimSpace(def))
	}
	return nil
}

func setBool(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.BoolVar(v.Addr().Interface().(*bool), val, v.Bool(), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			return parseBool(v, value)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseBool(v, strings.ToLower(def))
	}
	return nil
}

func setInt64(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.Int64Var(v.Addr().Interface().(*int64), val, v.Int(), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			return parseInt64(v, value)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseInt64(v, def)
	}
	return nil
}

func setInt(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.IntVar(v.Addr().Interface().(*int), val, int(v.Int()), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			return parseInt64(v, value)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseInt64(v, def)
	}
	return nil
}

func setFloat64(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.Float64Var(v.Addr().Interface().(*float64), val, v.Float(), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			return parseFloat64(v, value)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseFloat64(v, def)
	}
	return nil
}

func setDuration(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.DurationVar(v.Addr().Interface().(*time.Duration), val, time.Duration(v.Int()), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if raw := os.Getenv(val); raw != "" {
			return parseDuration(v, raw)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseDuration(v, def)
	}
	return nil
}

func parseDuration(v *reflect.Value, val string) error {
	d, err := time.ParseDuration(strings.TrimSpace(val))
	if err != nil {
		return err
	}
	v.SetInt(int64(d))
	return nil
}

func setStringSlice(v *reflect.Value, t reflect.StructTag) error {
	if _, ok := t.Lookup(readFlagKey); ok {
		return fmt.Errorf("flag tag is not supported for slice fields")
	}
	var raw string
	if val, ok := t.Lookup(readEnvKey); ok {
		raw = os.Getenv(val)
	}
	if raw == "" {
		if def, ok := t.Lookup(defaultKey); ok {
			raw = def
		}
	}
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	v.Set(reflect.ValueOf(result))
	return nil
}

func parseBool(v *reflect.Value, val string) error {
	if trimmed := strings.TrimSpace(val); trimmed != "" {
		parsed, err := strconv.ParseBool(trimmed)
		if err != nil {
			return err
		}
		v.SetBool(parsed)
	}
	return nil
}

func parseInt64(v *reflect.Value, val string) error {
	if trimmed := strings.TrimSpace(val); trimmed != "" {
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(parsed)
	}
	return nil
}

func parseFloat64(v *reflect.Value, val string) error {
	if trimmed := strings.TrimSpace(val); trimmed != "" {
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return err
		}
		v.SetFloat(parsed)
	}
	return nil
}

func getUsage(tag reflect.StructTag) string {
	val, _ := tag.Lookup(flagUsage)
	return val
}
