// Package gonphig loads configuration from multiple sources into a typed Go
// struct using struct tags. Sources are merged in a fixed priority order:
// CLI flags (highest) → environment variables → struct tag defaults → YAML
// file (lowest).
//
// The two entry points are ReadConfig and ReadFromFile. Both require a pointer
// to a struct and a *flag.FlagSet. Gonphig registers flags on the provided
// FlagSet but never calls Parse — the caller owns that step, ensuring gonphig
// is safe to use from libraries, CLIs, and tests without polluting global
// flag state.
//
// # Supported field types
//
// string, int, int64, float32, float64, bool, time.Duration, and []string are
// supported. []string values are read as comma-separated strings from env vars
// and the default tag. time.Duration values accept any string understood by
// time.ParseDuration (e.g. "5s", "1m30s").
//
// # Struct tags
//
//   - flag:"name"       bind to a CLI flag
//   - flag-usage:"txt"  usage string shown in --help (optional, use with flag)
//   - env:"VAR"         bind to an environment variable
//   - default:"val"     fallback when no flag or env var is set
//   - validate:"required" return an error if the field is the zero value after loading
//   - yaml:"name"       rename the field in a YAML source file
//
// Tags may be combined freely. When a field has both flag and default tags, the
// default value is used as the flag's default so --help shows meaningful output.
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

// ReadFromFile loads configuration into c from a YAML file at configPath,
// then overlays values from environment variables and registers CLI flags on
// fs, following the standard source priority (flags > env > default > YAML).
//
// The YAML file is the lowest-priority source. Any field bound to an env var
// or flag tag will override what the file provides.
//
// The caller must call fs.Parse after ReadFromFile returns so that registered
// flags are populated from the command line.
//
// c must be a non-nil pointer to a struct. Passing a non-pointer or a pointer
// to a non-struct type returns an error.
func ReadFromFile(configPath string, fs *flag.FlagSet, c any) error {
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(configFile, c); err != nil {
		return err
	}
	return ReadConfig(fs, c)
}

// ReadConfig loads configuration into c from environment variables and struct
// tag defaults, and registers CLI flags on fs.
//
// Source priority: flags (after fs.Parse) > env vars > default tag values.
// For YAML-based configuration use ReadFromFile instead.
//
// The caller must call fs.Parse after ReadConfig returns so that registered
// flags are populated from the command line. Pass flag.CommandLine to use the
// standard global flag set, or a custom *flag.FlagSet for isolation in tests
// or libraries.
//
// fs must not be nil. c must be a non-nil pointer to a struct. Passing a
// non-pointer or a pointer to a non-struct type returns an error. Unsupported
// field types (e.g. chan, func) return an error at load time.
//
// If any field is tagged validate:"required" and its value remains the zero
// value after all sources are applied, ReadConfig returns an error.
func ReadConfig(fs *flag.FlagSet, c any) error {
	if fs == nil {
		return errors.New("flag set must not be nil")
	}

	t := reflect.TypeOf(c)

	switch t.Kind() {
	case reflect.Ptr, reflect.Interface:
		val := t.Elem()
		if val.Kind() != reflect.Struct {
			return errors.New("invalid configuration structure")
		}
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

// durationType is used to detect time.Duration fields by named type before
// the reflect.Int64 case in the kind switch, since Duration's Kind() is Int64.
var durationType = reflect.TypeOf(time.Duration(0))

// overwriteFields applies tag-driven values (flag, env, default) to a single
// struct field. It recurses into nested structs. time.Duration is matched by
// named type before the kind switch to avoid being treated as a raw int64.
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
	case reflect.Float32:
		return overwriteValue(f, v, func(v *reflect.Value, t reflect.StructTag) error { return setFloat32(fs, v, t) })
	case reflect.Float64:
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

// overwriteValue calls setValue only when the field has at least one struct tag
// defined, skipping untagged fields entirely.
func overwriteValue(f reflect.StructField, v *reflect.Value, setValue func(*reflect.Value, reflect.StructTag) error) error {
	if len(f.Tag) > 0 {
		return setValue(v, f.Tag)
	}
	return nil
}

// setString applies the flag, env, or default tag to a string field.
// When a flag tag is present the default tag value (if any) is used as the
// flag's default so that --help displays a meaningful value.
func setString(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		def := v.String()
		if d, ok := t.Lookup(defaultKey); ok && d != "" {
			def = strings.TrimSpace(d)
		}
		fs.StringVar(v.Addr().Interface().(*string), val, def, getUsage(t))
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

// setBool applies the flag, env, or default tag to a bool field.
// When a flag tag is present the default tag value (if any) is used as the
// flag's default so that --help displays a meaningful value.
func setBool(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		def := v.Bool()
		if d, ok := t.Lookup(defaultKey); ok {
			if parsed, err := strconv.ParseBool(strings.TrimSpace(d)); err == nil {
				def = parsed
			}
		}
		fs.BoolVar(v.Addr().Interface().(*bool), val, def, getUsage(t))
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

// setInt64 applies the flag, env, or default tag to an int64 field.
// When a flag tag is present the default tag value (if any) is used as the
// flag's default so that --help displays a meaningful value.
func setInt64(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		def := v.Int()
		if d, ok := t.Lookup(defaultKey); ok {
			if parsed, err := strconv.ParseInt(strings.TrimSpace(d), 10, 64); err == nil {
				def = parsed
			}
		}
		fs.Int64Var(v.Addr().Interface().(*int64), val, def, getUsage(t))
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

// setInt applies the flag, env, or default tag to an int field.
// When a flag tag is present the default tag value (if any) is used as the
// flag's default so that --help displays a meaningful value.
func setInt(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		def := int(v.Int())
		if d, ok := t.Lookup(defaultKey); ok {
			if parsed, err := strconv.ParseInt(strings.TrimSpace(d), 10, 64); err == nil {
				def = int(parsed)
			}
		}
		fs.IntVar(v.Addr().Interface().(*int), val, def, getUsage(t))
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

// float32Flag implements flag.Value for float32 fields. The flag package has
// no Float32Var, so we use a custom flag.Value to avoid the unsafe pointer
// cast that would result from reinterpreting a *float32 as *float64.
type float32Flag struct{ v *reflect.Value }

func (f float32Flag) String() string {
	return strconv.FormatFloat(f.v.Float(), 'g', -1, 32)
}

func (f float32Flag) Set(s string) error {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if err != nil {
		return err
	}
	f.v.SetFloat(parsed)
	return nil
}

// setFloat32 applies the flag, env, or default tag to a float32 field.
// Flags use a custom flag.Value implementation because the flag package has no
// native Float32Var. When a flag tag is present the default tag value (if any)
// is used as the flag's default so that --help displays a meaningful value.
func setFloat32(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		if d, ok := t.Lookup(defaultKey); ok {
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(d), 32); err == nil {
				v.SetFloat(parsed)
			}
		}
		fs.Var(float32Flag{v}, val, getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			return parseFloat32(v, value)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseFloat32(v, def)
	}
	return nil
}

// setFloat64 applies the flag, env, or default tag to a float64 field.
// When a flag tag is present the default tag value (if any) is used as
// the flag's default so that --help displays a meaningful value.
func setFloat64(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		def := v.Float()
		if d, ok := t.Lookup(defaultKey); ok {
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(d), 64); err == nil {
				def = parsed
			}
		}
		fs.Float64Var(v.Addr().Interface().(*float64), val, def, getUsage(t))
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

// setDuration applies the flag, env, or default tag to a time.Duration field.
// Values are parsed with time.ParseDuration, accepting strings such as "5s",
// "1m30s", or "2h". When a flag tag is present the default tag value (if any)
// is used as the flag's default so that --help displays a meaningful value.
func setDuration(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		def := time.Duration(v.Int())
		if d, ok := t.Lookup(defaultKey); ok {
			if parsed, err := time.ParseDuration(strings.TrimSpace(d)); err == nil {
				def = parsed
			}
		}
		fs.DurationVar(v.Addr().Interface().(*time.Duration), val, def, getUsage(t))
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

// setStringSlice applies the env or default tag to a []string field.
// Values are parsed as a comma-separated list; whitespace around each entry
// is trimmed. The flag tag is not supported for slice fields and returns an
// error if present.
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

func parseFloat32(v *reflect.Value, val string) error {
	if trimmed := strings.TrimSpace(val); trimmed != "" {
		parsed, err := strconv.ParseFloat(trimmed, 32)
		if err != nil {
			return err
		}
		v.SetFloat(parsed)
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

func parseDuration(v *reflect.Value, val string) error {
	d, err := time.ParseDuration(strings.TrimSpace(val))
	if err != nil {
		return err
	}
	v.SetInt(int64(d))
	return nil
}

func getUsage(tag reflect.StructTag) string {
	val, _ := tag.Lookup(flagUsage)
	return val
}
