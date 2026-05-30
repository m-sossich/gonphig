// Package gonphig loads configuration from multiple sources into a typed Go
// struct using struct tags. Sources are merged in a fixed priority order:
// CLI flags (highest) → environment variables → .env file → struct tag
// defaults → YAML file (lowest).
//
// .env files are resolved via the env struct tag — fields without an env tag
// are not reachable from a .env file.
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
// time.ParseDuration (e.g. "5s", "1m30s"). In YAML, always use the string
// form — a bare integer zero (timeout: 0) is rejected; write timeout: 0s.
//
// # Struct tags
//
//   - flag:"name"         bind to a CLI flag (requires WithFlags or WithArgs)
//   - flag-usage:"txt"    usage string shown in --help (optional, use with flag)
//   - env:"VAR"           bind to an environment variable or .env key
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
	"github.com/m-sossich/gonphig/internal/parser"
	"github.com/m-sossich/gonphig/internal/validation"
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
	filePath string
	hasFile  bool
	fs       *flag.FlagSet
	args     []string
	hasFlags bool
}

// WithFile enables a file as a configuration source, dispatching to the
// appropriate parser based on the file extension (.yml/.yaml for YAML,
// .env for dotenv). File values are the lowest-priority source — they are
// overridden by env vars and flags.
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
// priority order: flags > env vars > .env file > struct tag defaults > file.
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
	if err := validateInput(c); err != nil {
		return err
	}
	s, err := buildSettings(opts)
	if err != nil {
		return err
	}
	l := &loader{fs: s.fs}
	if err := l.loadFile(c, s); err != nil {
		return err
	}
	if err := l.applyFields(c); err != nil {
		return err
	}
	if err := s.parseFlags(); err != nil {
		return err
	}
	return validation.ValidateRequired(c)
}

func validateInput(c any) error {
	if c == nil {
		return errors.New("configuration must not be nil")
	}
	switch reflect.TypeOf(c).Kind() {
	case reflect.Ptr:
		if reflect.TypeOf(c).Elem().Kind() != reflect.Struct {
			return errors.New("invalid configuration structure")
		}
	case reflect.Struct:
		return errors.New("configuration to load needs to be a pointer")
	default:
		return errors.New("invalid configuration structure")
	}
	return nil
}

func buildSettings(opts []Option) (*settings, error) {
	s := &settings{}
	for _, opt := range opts {
		opt(s)
	}
	if s.hasFlags && s.fs == nil {
		return nil, errors.New("flag set must not be nil")
	}
	if !s.hasFlags {
		s.fs = flag.NewFlagSet("", flag.ContinueOnError)
	}
	return s, nil
}

func (s *settings) parseFlags() error {
	if !s.hasFlags {
		return nil
	}
	return s.fs.Parse(s.args)
}

func (l *loader) loadFile(c any, s *settings) error {
	if !s.hasFile {
		return nil
	}
	parse, kind, err := parser.Lookup(s.filePath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	switch kind {
	case parser.KindStruct:
		return parse(data, c)
	case parser.KindKV:
		return parse(data, &l.dotenvVars)
	}
	return nil
}

func (l *loader) applyFields(c any) error {
	rv := reflect.ValueOf(c).Elem()
	rt := reflect.TypeOf(c).Elem()
	for i := 0; i < rt.NumField(); i++ {
		value := rv.Field(i)
		if err := l.overwriteFields(rt.Field(i), &value); err != nil {
			return err
		}
	}
	return nil
}

// loader carries per-Load context (FlagSet, dotenv values) so it does not
// need to be threaded through every setter function signature.
type loader struct {
	fs         *flag.FlagSet
	dotenvVars map[string]string
}

// applyTagSources resolves the default and env tag sources for v using parse.
// The default is applied only when v is zero; env always wins when the variable
// resolves to a non-empty string. Default parse errors are silently ignored;
// env parse errors are returned.
func (l *loader) applyTagSources(v *reflect.Value, t reflect.StructTag, parse func(string) error) error {
	if v.IsZero() {
		if def, ok := t.Lookup(defaultKey); ok && def != "" {
			_ = parse(def)
		}
	}
	if key, ok := t.Lookup(readEnvKey); ok {
		if raw := l.getenv(key); raw != "" {
			if err := parse(raw); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyField applies all tag sources (default, env, flag) to v.
// parse handles default and env values; registerFlag wires up the CLI flag
// using the current field value — already resolved from lower-priority sources
// — as the flag default.
func (l *loader) applyField(v *reflect.Value, t reflect.StructTag, parse func(string) error, registerFlag func(name, usage string)) error {
	if err := l.applyTagSources(v, t, parse); err != nil {
		return err
	}
	if name, ok := t.Lookup(readFlagKey); ok {
		registerFlag(name, getUsage(t))
	}
	return nil
}

// getenv looks up key in the environment, falling back to .env file values.
func (l *loader) getenv(key string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return l.dotenvVars[key]
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
	return l.applyField(v, t,
		func(s string) error { v.SetString(strings.TrimSpace(s)); return nil },
		func(name, usage string) { l.fs.StringVar(v.Addr().Interface().(*string), name, v.String(), usage) },
	)
}

func (l *loader) setBool(v *reflect.Value, t reflect.StructTag) error {
	return l.applyField(v, t,
		func(s string) error { return parseBool(v, s) },
		func(name, usage string) { l.fs.BoolVar(v.Addr().Interface().(*bool), name, v.Bool(), usage) },
	)
}

func (l *loader) setInt64(v *reflect.Value, t reflect.StructTag) error {
	return l.applyField(v, t,
		func(s string) error { return parseInt64(v, s) },
		func(name, usage string) { l.fs.Int64Var(v.Addr().Interface().(*int64), name, v.Int(), usage) },
	)
}

func (l *loader) setInt(v *reflect.Value, t reflect.StructTag) error {
	return l.applyField(v, t,
		func(s string) error { return parseInt64(v, s) },
		func(name, usage string) { l.fs.IntVar(v.Addr().Interface().(*int), name, int(v.Int()), usage) },
	)
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
	return l.applyField(v, t,
		func(s string) error { return parseFloat(v, s, 32) },
		func(name, usage string) { l.fs.Var(float32Flag{v}, name, usage) },
	)
}

func (l *loader) setFloat64(v *reflect.Value, t reflect.StructTag) error {
	return l.applyField(v, t,
		func(s string) error { return parseFloat(v, s, 64) },
		func(name, usage string) { l.fs.Float64Var(v.Addr().Interface().(*float64), name, v.Float(), usage) },
	)
}

func (l *loader) setDuration(v *reflect.Value, t reflect.StructTag) error {
	return l.applyField(v, t,
		func(s string) error { return parseDuration(v, s) },
		func(name, usage string) {
			l.fs.DurationVar(v.Addr().Interface().(*time.Duration), name, time.Duration(v.Int()), usage)
		},
	)
}

func (l *loader) setStringSlice(v *reflect.Value, t reflect.StructTag) error {
	if _, ok := t.Lookup(readFlagKey); ok {
		return fmt.Errorf("flag tag is not supported for slice fields")
	}
	raw := l.resolveSliceRaw(v, t)
	if raw == "" {
		return nil
	}
	v.Set(reflect.ValueOf(splitTrimmed(raw)))
	return nil
}

func (l *loader) resolveSliceRaw(v *reflect.Value, t reflect.StructTag) string {
	if key, ok := t.Lookup(readEnvKey); ok {
		if raw := l.getenv(key); raw != "" {
			return raw
		}
	}
	if v.IsNil() {
		if def, ok := t.Lookup(defaultKey); ok {
			return def
		}
	}
	return ""
}

func splitTrimmed(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
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
