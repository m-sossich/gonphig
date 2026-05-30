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
//   - flag:"name"         bind to a CLI flag (requires WithFlags or WithArgs)
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

// Option configures Load. Options are created by WithFile, WithArgs, WithFlags,
// and WithEnvPrefix.
type Option func(*settings)

type settings struct {
	filePath  string
	hasFile   bool
	fs        *flag.FlagSet
	args      []string
	hasFlags  bool
	envPrefix string
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

// WithArgs enables CLI flags as the highest-priority source. Gonphig creates
// a FlagSet internally, registers flags, and calls Parse(args). args is
// typically os.Args[1:].
//
// Use WithFlags instead if you need direct control over the FlagSet (custom
// error mode, registering your own flags on the same set).
func WithArgs(args []string) Option {
	return func(s *settings) {
		s.fs = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		s.args = args
		s.hasFlags = true
	}
}

// WithFlags enables CLI flags as the highest-priority source using a caller-
// provided FlagSet. Gonphig registers flags on fs, then calls fs.Parse(args).
// The caller may register additional flags on fs before calling Load.
//
// Use WithArgs for the common case where you do not need a custom FlagSet.
func WithFlags(fs *flag.FlagSet, args []string) Option {
	return func(s *settings) {
		s.fs = fs
		s.args = args
		s.hasFlags = true
	}
}

// WithEnvPrefix prepends prefix (uppercased, separated by "_") to every env
// var lookup. A field tagged env:"HOST" with WithEnvPrefix("APP") will look
// up APP_HOST in the environment.
func WithEnvPrefix(prefix string) Option {
	return func(s *settings) {
		s.envPrefix = strings.ToUpper(strings.TrimRight(prefix, "_"))
	}
}

// Bootstrap loads configuration into c exactly like Load, but panics on error.
// Intended for use in main functions where a config failure is unrecoverable.
//
//	gonphig.Bootstrap(&cfg, gonphig.WithEnvPrefix("APP"))
func Bootstrap(c any, opts ...Option) {
	if err := Load(c, opts...); err != nil {
		panic(err)
	}
}

// Load reads configuration into c from all enabled sources, applying them in
// priority order: flags > env vars > struct tag defaults > YAML file.
//
// Environment variables and struct tag defaults are always considered.
// Additional sources are enabled via WithFile, WithArgs, and WithFlags.
// Use WithEnvPrefix to apply a common prefix to all env var lookups.
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

	if s.hasFlags && s.fs == nil {
		return errors.New("flag set must not be nil")
	}
	if !s.hasFlags {
		s.fs = flag.NewFlagSet("", flag.ContinueOnError)
	}

	l := &loader{fs: s.fs, envPrefix: s.envPrefix}

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

	// Step 2: apply defaults, env vars, and register flags.
	rv := reflect.ValueOf(c).Elem()
	val := t.Elem()
	for i := 0; i < val.NumField(); i++ {
		value := rv.Field(i)
		if err := l.overwriteFields(val.Field(i), &value); err != nil {
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

// loader carries per-Load context (FlagSet, env prefix) so it does not need
// to be threaded through every setter function signature.
type loader struct {
	fs        *flag.FlagSet
	envPrefix string
}

// getenv looks up key in the environment, applying the env prefix when set.
func (l *loader) getenv(key string) string {
	if l.envPrefix != "" {
		return os.Getenv(l.envPrefix + "_" + key)
	}
	return os.Getenv(key)
}

// durationType is used to detect time.Duration fields by named type before
// the reflect.Int64 case in the kind switch, since Duration's Kind() is Int64.
var durationType = reflect.TypeOf(time.Duration(0))

// overwriteFields applies default, env, and flag tags to a single struct
// field in that order. It recurses into nested structs. Parse errors are
// wrapped with the field name so callers can identify which field failed.
func (l *loader) overwriteFields(f reflect.StructField, v *reflect.Value) error {
	if f.Type == durationType {
		return l.wrap(f.Name, overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error {
			return l.setDuration(v, t)
		}))
	}

	switch f.Type.Kind() {
	case reflect.Struct:
		for i := 0; i < f.Type.NumField(); i++ {
			value := v.Field(i)
			if err := l.overwriteFields(f.Type.Field(i), &value); err != nil {
				return err
			}
		}
	case reflect.Int64:
		return l.wrap(f.Name, overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error {
			return l.setInt64(v, t)
		}))
	case reflect.Int:
		return l.wrap(f.Name, overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error {
			return l.setInt(v, t)
		}))
	case reflect.Float32:
		return l.wrap(f.Name, overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error {
			return l.setFloat32(v, t)
		}))
	case reflect.Float64:
		return l.wrap(f.Name, overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error {
			return l.setFloat64(v, t)
		}))
	case reflect.String:
		return l.wrap(f.Name, overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error {
			return l.setString(v, t)
		}))
	case reflect.Bool:
		return l.wrap(f.Name, overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error {
			return l.setBool(v, t)
		}))
	case reflect.Slice:
		if f.Type.Elem().Kind() != reflect.String {
			return nil
		}
		return l.wrap(f.Name, overwriteValue(f.Tag, v, func(v *reflect.Value, t reflect.StructTag) error {
			return l.setStringSlice(v, t)
		}))
	case reflect.Map:
		return nil
	default:
		return fmt.Errorf("invalid field[%s] type[%s]", f.Name, f.Type.Name())
	}
	return nil
}

// wrap annotates err with the field name, making parse failures actionable.
func (l *loader) wrap(fieldName string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", fieldName, err)
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

func (l *loader) setString(v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok && def != "" {
			v.SetString(strings.TrimSpace(def))
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if variable := l.getenv(val); variable != "" {
			v.SetString(variable)
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		l.fs.StringVar(v.Addr().Interface().(*string), val, v.String(), getUsage(t))
	}
	return nil
}

func (l *loader) setBool(v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseBool(v, strings.ToLower(def))
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := l.getenv(val); value != "" {
			if err := parseBool(v, value); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		l.fs.BoolVar(v.Addr().Interface().(*bool), val, v.Bool(), getUsage(t))
	}
	return nil
}

func (l *loader) setInt64(v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseInt64(v, def)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := l.getenv(val); value != "" {
			if err := parseInt64(v, value); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		l.fs.Int64Var(v.Addr().Interface().(*int64), val, v.Int(), getUsage(t))
	}
	return nil
}

func (l *loader) setInt(v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseInt64(v, def)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := l.getenv(val); value != "" {
			if err := parseInt64(v, value); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		l.fs.IntVar(v.Addr().Interface().(*int), val, int(v.Int()), getUsage(t))
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

func (l *loader) setFloat32(v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseFloat(v, def, 32)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := l.getenv(val); value != "" {
			if err := parseFloat(v, value, 32); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		l.fs.Var(float32Flag{v}, val, getUsage(t))
	}
	return nil
}

func (l *loader) setFloat64(v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseFloat(v, def, 64)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if value := l.getenv(val); value != "" {
			if err := parseFloat(v, value, 64); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		l.fs.Float64Var(v.Addr().Interface().(*float64), val, v.Float(), getUsage(t))
	}
	return nil
}

func (l *loader) setDuration(v *reflect.Value, t reflect.StructTag) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok {
			_ = parseDuration(v, def)
		}
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		if raw := l.getenv(val); raw != "" {
			if err := parseDuration(v, raw); err != nil {
				return err
			}
		}
	}
	if val, ok := t.Lookup(readFlagKey); ok {
		l.fs.DurationVar(v.Addr().Interface().(*time.Duration), val, time.Duration(v.Int()), getUsage(t))
	}
	return nil
}

// setStringSlice applies default → env to a []string field.
// Values are parsed as a comma-separated list; whitespace around each entry
// is trimmed. The flag tag is not supported for slice fields.
func (l *loader) setStringSlice(v *reflect.Value, t reflect.StructTag) error {
	if _, ok := t.Lookup(readFlagKey); ok {
		return fmt.Errorf("flag tag is not supported for slice fields")
	}
	if !v.IsNil() {
		return nil
	}
	var raw string
	if val, ok := t.Lookup(readEnvKey); ok {
		raw = l.getenv(val)
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
