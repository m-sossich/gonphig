// Package gonphig loads configuration from multiple sources into a typed Go
// struct using struct tags. Sources are merged in a fixed priority order:
// CLI flags (highest) → environment variables → struct tag defaults → YAML
// file (lowest).
//
// The single entry point is Load. Environment variables and struct tag
// defaults are always considered. Additional sources — YAML files and CLI
// flags — are enabled via options.
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
//   - flag:"name"         bind to a CLI flag (requires WithFlags option)
//   - flag-usage:"txt"    usage string shown in --help (optional, use with flag)
//   - env:"VAR"           bind to an environment variable
//   - default:"val"       fallback when no higher-priority source sets the field
//   - validate:"required" return an error if the field is zero after loading
//   - yaml:"name"         rename the field when reading from a YAML file
//
// Tags may be combined freely on the same field.
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

// Option configures Load. Options are created by WithFile and WithFlags.
type Option func(*settings)

type settings struct {
	filePath string
	hasFile  bool
	fs       *flag.FlagSet
	args     []string
	hasFlags bool
}

// WithFile enables a YAML file as a configuration source.
// YAML is the lowest-priority source — it is overridden by env vars, struct
// tag defaults, and flags.
func WithFile(path string) Option {
	return func(s *settings) {
		s.filePath = path
		s.hasFile = true
	}
}

// WithFlags enables CLI flags as the highest-priority source.
// Gonphig registers flags on fs, then calls fs.Parse(args) internally.
// args is typically os.Args[1:].
// The caller may register additional flags on fs before calling Load.
func WithFlags(fs *flag.FlagSet, args []string) Option {
	return func(s *settings) {
		s.fs = fs
		s.args = args
		s.hasFlags = true
	}
}

// Load reads configuration into c from all enabled sources, applying them in
// priority order: flags > env vars > struct tag defaults > YAML file.
//
// Environment variables and struct tag defaults are always considered.
// Additional sources are enabled via WithFile and WithFlags.
//
// c must be a non-nil pointer to a struct. Passing nil, a non-pointer, or a
// pointer to a non-struct type returns an error. Unsupported field types
// (e.g. chan, func) return an error at load time.
//
// If any field is tagged validate:"required" and its value remains zero after
// all sources are applied, Load returns an error.
func Load(c any, opts ...Option) error {
	if c == nil {
		return errors.New("configuration must not be nil")
	}

	t := reflect.TypeOf(c)
	switch t.Kind() {
	case reflect.Ptr:
		if t.Elem().Kind() != reflect.Struct {
			return errors.New("invalid configuration structure")
		}
	case reflect.Struct:
		return errors.New("configuration to load needs to be a pointer")
	default:
		return errors.New("invalid configuration structure")
	}

	s := &settings{}
	for _, opt := range opts {
		opt(s)
	}

	// Use a throwaway FlagSet when no WithFlags option is provided.
	// Flags registered on it are never parsed, so flag tags have no effect.
	if !s.hasFlags {
		s.fs = flag.NewFlagSet("", flag.ContinueOnError)
	}

	// Step 1: load YAML as the baseline (lowest priority).
	if s.hasFile {
		data, err := os.ReadFile(s.filePath)
		if err != nil {
			return err
		}
		if err = yaml.Unmarshal(data, c); err != nil {
			return err
		}
	}

	// Step 2: apply defaults, env vars, and register flags — in that order
	// so that each step can override the previous one. After registration,
	// Parse will let flag values override env values.
	rv := reflect.ValueOf(c).Elem()
	val := t.Elem()
	for i := 0; i < val.NumField(); i++ {
		value := rv.Field(i)
		if err := overwriteFields(s.fs, val.Field(i), &value); err != nil {
			return err
		}
	}

	// Step 3: parse flags so CLI values override everything set so far.
	if s.hasFlags {
		if err := s.fs.Parse(s.args); err != nil {
			return err
		}
	}

	return validation.ValidateRequired(c)
}

// durationType is used to detect time.Duration fields by named type before
// the reflect.Int64 case in the kind switch, since Duration's Kind() is Int64.
var durationType = reflect.TypeOf(time.Duration(0))

// overwriteFields applies default, env, and flag tags to a single struct
// field in that order. It recurses into nested structs. time.Duration is
// matched by named type before the kind switch to avoid being treated as int64.
func overwriteFields(fs *flag.FlagSet, f reflect.StructField, v *reflect.Value) error {
	if f.Type == durationType {
		return overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error { return setDuration(fs, v, t) })
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
		return overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error { return setInt64(fs, v, t) })
	case reflect.Int:
		return overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error { return setInt(fs, v, t) })
	case reflect.Float32:
		return overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error { return setFloat32(fs, v, t) })
	case reflect.Float64:
		return overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error { return setFloat64(fs, v, t) })
	case reflect.String:
		return overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error { return setString(fs, v, t) })
	case reflect.Bool:
		return overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error { return setBool(fs, v, t) })
	case reflect.Slice:
		if f.Type.Elem().Kind() != reflect.String {
			return nil
		}
		return overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error { return setStringSlice(fs, v, t) })
	case reflect.Map:
		return nil
	default:
		return fmt.Errorf("invalid field[%s] type[%s]", f.Name, f.Type.Name())
	}
	return nil
}

// overwriteValue calls setValue only when the field has at least one struct
// tag defined, skipping untagged fields entirely.
func overwriteValue(tag reflect.StructTag, v *reflect.Value, setValue func(*reflect.Value, reflect.StructTag) error) error {
	if len(tag) > 0 {
		return setValue(v, tag)
	}
	return nil
}

// setString applies default → env → flag registration to a string field.
// The default is only applied when the field is still the zero value, so
// YAML-loaded values are not overwritten by defaults.
func setString(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok && def != "" {
			v.SetString(strings.TrimSpace(def))
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if variable := os.Getenv(val); variable != "" {
			v.SetString(variable)
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.StringVar(v.Addr().Interface().(*string), val, v.String(), getUsage(t))
	}
	return nil
}

// setBool applies default → env → flag registration to a bool field.
func setBool(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseBool(v, strings.ToLower(def))
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			if err := parseBool(v, value); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.BoolVar(v.Addr().Interface().(*bool), val, v.Bool(), getUsage(t))
	}
	return nil
}

// setInt64 applies default → env → flag registration to an int64 field.
func setInt64(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseInt64(v, def)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			if err := parseInt64(v, value); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.Int64Var(v.Addr().Interface().(*int64), val, v.Int(), getUsage(t))
	}
	return nil
}

// setInt applies default → env → flag registration to an int field.
func setInt(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseInt64(v, def)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			if err := parseInt64(v, value); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.IntVar(v.Addr().Interface().(*int), val, int(v.Int()), getUsage(t))
	}
	return nil
}

// float32Flag implements flag.Value for float32 fields. The flag package has
// no Float32Var, so a custom flag.Value is used instead. Unlike fs.XxxVar
// which writes the default directly, fs.Var only reads String() for display —
// so setFloat32 writes the default/env value to the field before registering
// the flag, then String() reflects that value back for --help output.
type float32Flag struct{ v *reflect.Value }

func (f float32Flag) String() string {
	return strconv.FormatFloat(f.v.Float(), 'g', -1, 32)
}

func (f float32Flag) Set(s string) error {
	return parseFloat(f.v, s, 32)
}

// setFloat32 applies default → env → flag registration to a float32 field.
func setFloat32(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseFloat(v, def, 32)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			if err := parseFloat(v, value, 32); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.Var(float32Flag{v}, val, getUsage(t))
	}
	return nil
}

// setFloat64 applies default → env → flag registration to a float64 field.
func setFloat64(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseFloat(v, def, 64)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := os.Getenv(val); value != "" {
			if err := parseFloat(v, value, 64); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.Float64Var(v.Addr().Interface().(*float64), val, v.Float(), getUsage(t))
	}
	return nil
}

// setDuration applies default → env → flag registration to a time.Duration
// field. Values are parsed with time.ParseDuration (e.g. "5s", "1m30s").
func setDuration(fs *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseDuration(v, def)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if raw := os.Getenv(val); raw != "" {
			if err := parseDuration(v, raw); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		fs.DurationVar(v.Addr().Interface().(*time.Duration), val, time.Duration(v.Int()), getUsage(t))
	}
	return nil
}

// setStringSlice applies default → env to a []string field.
// Values are parsed as a comma-separated list; whitespace around each entry
// is trimmed. The flag tag is not supported for slice fields and returns an
// error if present.
func setStringSlice(_ *flag.FlagSet, v *reflect.Value, t reflect.StructTag) error {
	if _, ok := t.Lookup(readFlagKey); ok {
		return fmt.Errorf("flag tag is not supported for slice fields")
	}
	// Only apply default/env if YAML hasn't already populated the slice.
	if !v.IsNil() {
		return nil
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

// parseFloat parses val into a float of the given bitSize (32 or 64) and sets
// it on v. Used by both setFloat32 and setFloat64 to avoid duplication.
func parseFloat(v *reflect.Value, val string, bitSize int) error {
	if trimmed := strings.TrimSpace(val); trimmed != "" {
		parsed, err := strconv.ParseFloat(trimmed, bitSize)
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
