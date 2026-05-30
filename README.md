<div align="center">
    <img src="https://raw.githubusercontent.com/m-sossich/gonphig/main/.github/logo.png" width="300">
</div><br/>

[![Go](https://github.com/m-sossich/gonphig/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/m-sossich/gonphig/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/m-sossich/gonphig)](https://goreportcard.com/report/github.com/m-sossich/gonphig)
[![Go Reference](https://pkg.go.dev/badge/github.com/m-sossich/gonphig/pkg/gonphig.svg)](https://pkg.go.dev/github.com/m-sossich/gonphig/pkg/gonphig)
[![codecov](https://codecov.io/gh/m-sossich/gonphig/branch/main/graph/badge.svg)](https://codecov.io/gh/m-sossich/gonphig)

## What is this for?

Gonphig loads configuration from multiple sources into a typed Go struct using struct tags. Sources are merged in a fixed priority order so you never manually stitch values together.

## Installation

```sh
go get github.com/m-sossich/gonphig
```

## Quick start

```go
type Config struct {
    Host    string        `env:"HOST"    default:"localhost"`
    Port    int           `env:"PORT"    default:"8080"`
    Timeout time.Duration `env:"TIMEOUT" default:"30s"`
    APIKey  string        `env:"API_KEY" validate:"required"`
}

var cfg Config
if err := gonphig.Load(&cfg, gonphig.WithEnvPrefix("APP")); err != nil {
    log.Fatal(err)
}
```

Or panic on error in `main`:

```go
gonphig.Bootstrap(&cfg, gonphig.WithEnvPrefix("APP"))
```

## Source priority

Sources are evaluated in this order — higher entries win:

| Priority | Source | How to enable |
|----------|--------|---------------|
| 1 (highest) | CLI flag | `WithArgs(args)` or `WithFlags(fs, args)` + `flag:"name"` tag |
| 2 | Environment variable | always on — `env:"VAR"` tag |
| 3 | Struct tag default | always on — `default:"value"` tag |
| 4 (lowest) | YAML file | `WithFile("path")` option |

## How to use it

### Env vars and defaults

Environment variables and struct tag defaults require no options — they are always active.

```go
type Config struct {
    Host  string `env:"HOST"  default:"localhost"`
    Port  int    `env:"PORT"  default:"8080"`
    Debug bool   `env:"DEBUG" default:"false"`
}

var cfg Config
if err := gonphig.Load(&cfg); err != nil {
    log.Fatal(err)
}
```

### Env prefix

Use `WithEnvPrefix` to prepend a common prefix to all env var lookups. A field tagged `env:"HOST"` with `WithEnvPrefix("APP")` reads `APP_HOST` from the environment.

```go
// reads APP_HOST, APP_PORT, APP_DEBUG
if err := gonphig.Load(&cfg, gonphig.WithEnvPrefix("APP")); err != nil {
    log.Fatal(err)
}
```

### YAML file

YAML values are the lowest-priority source — env vars and flags always override them.

```go
if err := gonphig.Load(&cfg,
    gonphig.WithFile("config.yml"),
    gonphig.WithEnvPrefix("APP"),
); err != nil {
    log.Fatal(err)
}
```

Use the `yaml` tag to map a struct field to a differently named YAML key:

```go
type Config struct {
    DatabaseURL string `yaml:"database_url" env:"DATABASE_URL"`
}
```

### CLI flags

Use `WithArgs` for the simple case — gonphig creates and manages the `FlagSet` internally:

```go
type Config struct {
    Host string `flag:"host" flag-usage:"server hostname" default:"localhost"`
    Port int    `flag:"port" flag-usage:"server port"     default:"8080"`
}

var cfg Config
if err := gonphig.Load(&cfg, gonphig.WithArgs(os.Args[1:])); err != nil {
    log.Fatal(err)
}
```

Use `WithFlags` when you need control over the `FlagSet` (custom error mode, registering your own flags alongside gonphig's):

```go
fs := flag.NewFlagSet("myapp", flag.ExitOnError)
fs.String("log-level", "info", "log verbosity") // your own flag

var cfg Config
if err := gonphig.Load(&cfg, gonphig.WithFlags(fs, os.Args[1:])); err != nil {
    log.Fatal(err)
}
```

### Bootstrap

For `main` functions where a config failure is unrecoverable, `Bootstrap` panics instead of returning an error:

```go
gonphig.Bootstrap(&cfg,
    gonphig.WithFile("config.yml"),
    gonphig.WithEnvPrefix("APP"),
    gonphig.WithArgs(os.Args[1:]),
)
```

### Nested structs

Gonphig recurses into nested structs. All tags work at any level of nesting.

```go
type Config struct {
    Server struct {
        Host string `env:"SERVER_HOST" default:"localhost"`
        Port int    `env:"SERVER_PORT" default:"8080"`
    }
    DB struct {
        URL string `env:"DB_URL" validate:"required"`
    }
}
```

### Validation

Use `validate:"required"` to require a field to be explicitly set. A required field that holds its zero value (`""`, `0`, `0s`) after all sources are applied will cause `Load` to return an error.

> **Note:** `validate:"required"` is not supported on `bool` fields — `false` is a valid value that cannot be distinguished from unset. Applying it to a `bool` returns an error at load time.

```go
type Config struct {
    APIKey  string        `env:"API_KEY"  validate:"required"`
    Port    int           `env:"PORT"     validate:"required"`
    Timeout time.Duration `env:"TIMEOUT"  validate:"required"`
}
```

## Supported field types

| Go type         | `env` | `flag` | `default` | `validate:"required"` |
|-----------------|:-----:|:------:|:---------:|:---------------------:|
| `string`        | ✓     | ✓      | ✓         | ✓                     |
| `int`           | ✓     | ✓      | ✓         | ✓                     |
| `int64`         | ✓     | ✓      | ✓         | ✓                     |
| `float32`       | ✓     | ✓      | ✓         | ✓                     |
| `float64`       | ✓     | ✓      | ✓         | ✓                     |
| `bool`          | ✓     | ✓      | ✓         | —                     |
| `time.Duration` | ✓     | ✓      | ✓         | ✓                     |
| `[]string`      | ✓     | —      | ✓         | ✓                     |
| `map`           | —     | —      | —         | —                     |

`[]string` values are comma-separated. Whitespace around entries is trimmed automatically:
```
HOSTS=host1, host2, host3  →  []string{"host1", "host2", "host3"}
```

Unsupported field types (channels, funcs, etc.) return an error at load time.

## Error handling

`Load` returns a non-nil error when:

- A `validate:"required"` field is not set
- An env var or default value cannot be parsed — error includes the field name
- An invalid flag value is passed
- A non-existent file path is provided to `WithFile`
- The config argument is nil, not a pointer, or a pointer to a non-struct type
- A nil `FlagSet` is passed to `WithFlags`

## External resources

- [YAML spec](https://yaml.org/spec/1.2/spec.html)
- [go-yaml](https://github.com/go-yaml/yaml)
