# <p align="center"><img src="https://raw.githubusercontent.com/m-sossich/gonphig/main/.github/logo.png" width="300"></p>
[![Go](https://github.com/m-sossich/gonphig/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/m-sossich/gonphig/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/m-sossich/gonphig)](https://goreportcard.com/report/github.com/m-sossich/gonphig)
[![](https://godoc.org/github.com/tendermint/iavl?status.svg)](https://pkg.go.dev/github.com/m-sossich/gonphig/pkg/gonphig)
[![codecov](https://codecov.io/gh/m-sossich/gonphig/branch/main/graph/badge.svg)](https://codecov.io/gh/m-sossich/gonphig)

## What is this for?

Gonphig loads configuration from multiple sources (flags, environment variables, YAML files, and struct tag defaults) into a typed Go struct. Sources are merged in a defined priority order so you never manually stitch values together.

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
    Debug   bool          `env:"DEBUG"`
    APIKey  string        `env:"API_KEY" validate:"required"`
}

var cfg Config
if err := gonphig.ReadConfig(flag.CommandLine, &cfg); err != nil {
    log.Fatal(err)
}
flag.Parse()
```

## Source priority

Sources are evaluated in this order — higher entries win:

| Priority | Source | How to bind |
|----------|--------|-------------|
| 1 (highest) | CLI flag | `flag:"flag-name"` tag |
| 2 | Environment variable | `env:"VAR_NAME"` tag |
| 3 | Default value | `default:"value"` tag |
| 4 (lowest) | YAML file | field name or `yaml:"name"` tag |

## How to use it

### Flags

Use the `flag` tag to bind a field to a CLI flag. Gonphig registers the flags on the provided `FlagSet` — **you are responsible for calling `Parse`** after `ReadConfig` returns.

```go
type Config struct {
    Host string `flag:"host" flag-usage:"server hostname"`
    Port int    `flag:"port" flag-usage:"server port" default:"8080"`
}

fs := flag.NewFlagSet("myapp", flag.ExitOnError)

var cfg Config
if err := gonphig.ReadConfig(fs, &cfg); err != nil {
    log.Fatal(err)
}
fs.Parse(os.Args[1:])
```

For simple `main` programs you can use `flag.CommandLine`:

```go
if err := gonphig.ReadConfig(flag.CommandLine, &cfg); err != nil {
    log.Fatal(err)
}
flag.Parse()
```

> **Why you own `Parse`:** gonphig is a library. Calling `flag.Parse()` inside a library hijacks the host application's flag set. By handing the `FlagSet` back to you, gonphig can be used safely in libraries, CLIs, and test suites without global side effects.

### Environment variables

Use the `env` tag to bind a field to an environment variable.

```go
type Config struct {
    Host   string `env:"HOST"`
    Port   int    `env:"PORT"`
    Debug  bool   `env:"DEBUG"`
}

var cfg Config
if err := gonphig.ReadConfig(flag.CommandLine, &cfg); err != nil {
    log.Fatal(err)
}
```

### Default values

Use the `default` tag as a fallback when no flag or env var is set. Tags can be combined freely.

```go
type Config struct {
    Host    string        `env:"HOST"    default:"localhost"`
    Port    int           `env:"PORT"    default:"8080"`
    Timeout time.Duration `env:"TIMEOUT" default:"30s"`
    Debug   bool          `env:"DEBUG"   default:"false"`
}
```

When a field also has a `flag` tag, the `default` value is used as the flag's default — so `--help` shows meaningful defaults.

### YAML file

Use `ReadFromFile` to seed configuration from a YAML file. YAML values are the lowest-priority source and will be overridden by env vars and flags.

```go
var cfg Config
if err := gonphig.ReadFromFile("config.yml", flag.CommandLine, &cfg); err != nil {
    log.Fatal(err)
}
flag.Parse()
```

Use the `yaml` tag to map a struct field to a differently named YAML key:

```go
type Config struct {
    DatabaseURL string `yaml:"database_url" env:"DATABASE_URL"`
}
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

Use `validate:"required"` to require a field to be explicitly set. A required field that holds its zero value (`""`, `0`, `false`, `0s`) will cause `ReadConfig` to return an error.

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
| `bool`          | ✓     | ✓      | ✓         | ✓                     |
| `time.Duration` | ✓     | ✓      | ✓         | ✓                     |
| `[]string`      | ✓     | —      | ✓         | ✓                     |
| `map`           | —     | —      | —         | —                     |

`[]string` values are comma-separated. Whitespace around entries is trimmed automatically:
```
HOSTS=host1, host2, host3  →  []string{"host1", "host2", "host3"}
```

Unsupported field types (channels, funcs, etc.) return an error at load time.

## Error handling

`ReadConfig` and `ReadFromFile` return a non-nil error when:

- A `validate:"required"` field is not set
- An env var or default value cannot be parsed into the field's type
- An invalid flag value is passed (when using `flag.ExitOnError`, the program exits)
- A non-existent YAML file path is provided
- The config argument is not a pointer to a struct

## External resources

- [YAML spec](https://yaml.org/spec/1.2/spec.html)
- [go-yaml](https://github.com/go-yaml/yaml)
