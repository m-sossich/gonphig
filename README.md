<div align="center">
    <img src="https://github.com/user-attachments/assets/89939d12-238d-45ff-bbb8-179a394228a9" width="300">
</div><br/>

[![Go](https://github.com/m-sossich/gonphig/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/m-sossich/gonphig/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/m-sossich/gonphig)](https://goreportcard.com/report/github.com/m-sossich/gonphig)
[![Go Reference](https://pkg.go.dev/badge/github.com/m-sossich/gonphig/pkg/gonphig.svg)](https://pkg.go.dev/github.com/m-sossich/gonphig/pkg/gonphig)

---

Gonphig loads configuration from multiple sources — environment variables, `.env` files, YAML files, and CLI flags — into a typed Go struct using struct tags. Sources are merged in a fixed priority order, so you never manually stitch values together or worry about which source wins.

## Installation

```sh
go get github.com/m-sossich/gonphig
```

## Quick start

```go
import "github.com/m-sossich/gonphig/pkg/gonphig"

type Config struct {
    Host    string        `env:"HOST"    default:"localhost"`
    Port    int           `env:"PORT"    default:"8080"`
    Timeout time.Duration `env:"TIMEOUT" default:"30s"`
    APIKey  string        `env:"API_KEY" validate:"required"`
}

var cfg Config
if err := gonphig.Load(&cfg); err != nil {
    log.Fatal(err)
}
```

For `main` functions where a config failure is fatal, use `Bootstrap` — it panics instead of returning an error:

```go
gonphig.Bootstrap(&cfg)
```

---

## Source priority

When the same field has a value from multiple sources, the higher-priority source always wins. Sources are evaluated in this order:

| Priority | Source | How to enable |
|:--------:|--------|---------------|
| 1 (highest) | CLI flag | `WithArgs(args)` or `WithFlags(fs, args)` + `flag:"name"` tag |
| 2 | Environment variable | always on — requires `env:"VAR"` tag |
| 3 | `.env` file | `WithFile(".env")` + `env:"VAR"` tag |
| 4 | Struct tag default | always on — `default:"value"` tag |
| 5 (lowest) | YAML file | `WithFile("config.yml")` |

Environment variables and struct tag defaults are always active — no option required. Every other source is opt-in via an option.

---

## Struct tags

Tags are the contract between your struct and gonphig. They can be combined freely on the same field.

| Tag | Purpose |
|-----|---------|
| `env:"VAR"` | Bind to an environment variable (or `.env` key) |
| `default:"value"` | Fallback value when no higher-priority source provides one |
| `flag:"name"` | Bind to a CLI flag (requires `WithArgs` or `WithFlags`) |
| `flag-usage:"text"` | Usage string shown in `--help` output (used with `flag`) |
| `yaml:"key"` | Map to a differently named key in a YAML file |
| `validate:"required"` | Return an error if the field is still zero after all sources are applied |

**Example — all tags on one field:**

```go
type Config struct {
    Host string `env:"HOST" default:"localhost" flag:"host" flag-usage:"server address" yaml:"host" validate:"required"`
}
```

---

## Sources in detail

### Environment variables

No option required. Gonphig reads from the OS environment for every field that has an `env` tag.

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

### YAML file

Pass any `.yml` or `.yaml` path to `WithFile`. YAML is the lowest-priority source — env vars, `.env` files, defaults, and flags always override it.

```go
if err := gonphig.Load(&cfg, gonphig.WithFile("config.yml")); err != nil {
    log.Fatal(err)
}
```

Use the `yaml` tag to map a struct field to a differently named YAML key:

```go
type Config struct {
    DatabaseURL string `yaml:"database_url" env:"DATABASE_URL"`
}
```

```yaml
# config.yml
database_url: "postgres://localhost:5432/mydb"
```

YAML reaches fields by struct field name or `yaml` tag regardless of whether an `env` tag is present. Nested struct fields are populated via YAML nesting:

```yaml
server:
  host: localhost
  port: 8080
db:
  url: postgres://localhost/mydb
```

```go
type Config struct {
    Server struct {
        Host string `yaml:"host"`
        Port int    `yaml:"port"`
    } `yaml:"server"`
    DB struct {
        URL string `yaml:"url"`
    } `yaml:"db"`
}
```

> **Note:** For `time.Duration` fields in YAML, always use the string form (`30s`, `1m30s`). A bare integer zero (`timeout: 0`) is rejected — write `timeout: 0s` instead.

### .env file

Pass a `.env` path to `WithFile`. Dotenv values sit between real environment variables and struct tag defaults: real env vars always win, defaults apply only when neither the OS environment nor the `.env` file provides a value.

```go
if err := gonphig.Load(&cfg, gonphig.WithFile(".env")); err != nil {
    log.Fatal(err)
}
```

`.env` files are resolved via the `env` tag. Fields without an `env` tag are not reachable from a `.env` file.

**Supported syntax:**

```sh
# comment — ignored
HOST=localhost
PORT=8080

export API_KEY=secret   # export prefix is stripped
EMPTY_KEY=              # empty value — treated as not set, field uses next priority
```

Quotes and variable expansion (`$VAR`) are not supported.

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

```sh
./myapp --host=production.example.com --port=9090
```

Use `WithFlags` when you need direct control over the `FlagSet` — for example, to set a custom error mode or to register your own flags on the same set:

```go
fs := flag.NewFlagSet("myapp", flag.ExitOnError)
fs.String("log-level", "info", "log verbosity")  // your own flag

var cfg Config
if err := gonphig.Load(&cfg, gonphig.WithFlags(fs, os.Args[1:])); err != nil {
    log.Fatal(err)
}
```

Flags registered via gonphig use the current field value — already resolved from lower-priority sources — as their default. This means `--help` always shows the effective default, not the zero value.

`[]string` fields do not support the `flag` tag — use `env` or `default` instead.

### Combining sources

All options can be combined. Gonphig applies them in priority order regardless of the order they are passed:

```go
var cfg Config
gonphig.Bootstrap(&cfg,
    gonphig.WithFile("config.yml"),   // lowest priority
    gonphig.WithArgs(os.Args[1:]),    // highest priority
)
```

---

## Nested structs

Gonphig recurses into nested structs. Every tag works at any level of nesting.

```go
type Config struct {
    Server struct {
        Host string        `env:"SERVER_HOST" default:"localhost"`
        Port int           `env:"SERVER_PORT" default:"8080"`
        TLS  bool          `env:"SERVER_TLS"  default:"false"`
    }
    DB struct {
        URL     string        `env:"DB_URL"     validate:"required"`
        MaxConn int           `env:"DB_MAX_CONN" default:"10"`
        Timeout time.Duration `env:"DB_TIMEOUT"  default:"5s"`
    }
    Auth struct {
        APIKey string `env:"AUTH_API_KEY" validate:"required"`
    }
}
```

---

## Validation

Tag any field with `validate:"required"` to require it to be explicitly set. A required field whose value is still zero (`""`, `0`, `false`, `0s`) after all sources are applied causes `Load` to return an error.

```go
type Config struct {
    APIKey  string        `env:"API_KEY"  validate:"required"`
    Port    int           `env:"PORT"     validate:"required"`
    Timeout time.Duration `env:"TIMEOUT"  validate:"required"`
}
```

**Constraints:**

- `validate:"required"` is **not supported on `bool` fields** — `false` is a valid intentional value that cannot be distinguished from unset. Using it on a `bool` returns an error at load time.
- Unknown rules (e.g., `validate:"requried"`) return an error immediately so typos fail loudly rather than being silently ignored.

---

## Supported field types

| Go type         | `env` / `.env` | `flag` | `default` | YAML | `validate:"required"` |
|-----------------|:--------------:|:------:|:---------:|:----:|:---------------------:|
| `string`        | ✓ | ✓ | ✓ | ✓ | ✓ |
| `int`           | ✓ | ✓ | ✓ | ✓ | ✓ |
| `int64`         | ✓ | ✓ | ✓ | ✓ | ✓ |
| `float32`       | ✓ | ✓ | ✓ | ✓ | ✓ |
| `float64`       | ✓ | ✓ | ✓ | ✓ | ✓ |
| `bool`          | ✓ | ✓ | ✓ | ✓ | — |
| `time.Duration` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `[]string`      | ✓ | — | ✓ | ✓ | ✓ |
| `map`           | — | — | — | — | — |

**`[]string`** — comma-separated in env vars, `.env` files, and `default` tags. Whitespace around entries is trimmed automatically. Loaded as a list from YAML.

```
HOSTS=host1, host2, host3  →  []string{"host1", "host2", "host3"}
```

**`time.Duration`** — accepts any string understood by `time.ParseDuration` (`"5s"`, `"300ms"`, `"1m30s"`) in all sources. In YAML, always use the string form — `timeout: 30s`, not `timeout: 0` (bare integer zero is rejected; write `timeout: 0s`).

**`bool`** — accepts `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, `False` from all string sources.

**`map`** — silently ignored. No error is returned; the field is left as nil.

Unsupported field types (`chan`, `func`, etc.) return an error at load time.

---

## Error handling

`Load` returns a descriptive, non-nil error when:

| Condition | Error message |
|-----------|---------------|
| `validate:"required"` field is zero after loading | `missing required configuration: <FieldName>` |
| `validate:"required"` on a `bool` field | `validate:"required" is not supported on bool field <FieldName>` |
| Unknown `validate` rule | `unknown validation rule "<rule>" on field <FieldName>` |
| Parse failure on an env var or default | `<FieldName>: <parse error>` |
| Invalid flag value | standard `flag` package error |
| File path does not exist | `open <path>: no such file or directory` |
| Unsupported file extension | `unsupported file format: "<ext>"` |
| Nil config | `configuration must not be nil` |
| Non-pointer config | `configuration to load needs to be a pointer` |
| Pointer to non-struct | `invalid configuration structure` |
| Nil `FlagSet` passed to `WithFlags` | `flag set must not be nil` |
| `flag` tag on a `[]string` field | `flag tag is not supported for slice fields` |
| Unsupported field type (`chan`, `func`, …) | `invalid field[<Name>] type[<type>]` |

Parse errors always include the field name, making it straightforward to identify which value in which source failed.

---

## Design decisions

**Single entry point.** `WithFile` accepts `.yml`, `.yaml`, and `.env` paths — the format is detected from the extension. Adding a new file format requires only a new parser function and one registry entry; the public API never changes.

**Fixed priority, no surprises.** The order flags > env > `.env` > defaults > YAML is hardcoded. There is no API to reorder it. This makes the library predictable: you always know which source wins without reading documentation.

**`.env` requires `env` tags.** Dotenv is an env-var-style source. It flows through the same env resolution pipeline as OS environment variables, so it can only reach fields that declare an `env` tag. Fields with only a `yaml` tag are not reachable from a `.env` file. This is intentional — if you want a field reachable from both YAML and dotenv, tag it with both.

**`bool` and `validate:"required"` are incompatible.** `false` is the zero value for `bool` AND a valid intentional configuration value. There is no way to distinguish "not set" from "explicitly set to false", so requiring a bool to be set is a meaningless constraint. Gonphig returns an error at load time if you try.

**No env var prefix.** The `env` tag holds the full, exact env var name — what you write is what gets looked up. This makes structs self-documenting: you can read any field and know exactly what env var it reads without needing to track a runtime prefix option.

---

## External resources

- [go-yaml v3](https://github.com/go-yaml/yaml)
- [YAML spec](https://yaml.org/spec/1.2/spec.html)
- [time.ParseDuration](https://pkg.go.dev/time#ParseDuration)
