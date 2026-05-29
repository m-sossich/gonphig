# <p align="center"><img src="https://raw.githubusercontent.com/m-sossich/gonphig/master/.github/logo.png" width="300"></p>
![Go](https://github.com/m-sossich/gonphig/workflows/Go/badge.svg?branch=master) 
[![Go Report Card](https://goreportcard.com/badge/github.com/m-sossich/gonphig)](https://goreportcard.com/report/github.com/m-sossich/gonphig)
[![](https://godoc.org/github.com/tendermint/iavl?status.svg)](https://pkg.go.dev/github.com/m-sossich/gonphig/pkg/gonphig)
[![codecov](https://codecov.io/gh/m-sossich/gonphig/branch/master/graph/badge.svg)](https://codecov.io/gh/m-sossich/gonphig)

## What is this for?
Read configurations from multiple sources into a typed struct using struct tags.

## How to use it

### Read configuration from flags

Use the `flag` tag to bind a field to a CLI flag. You are responsible for creating the `FlagSet` and calling `Parse` — gonphig only registers the flags.

```go
type Conf struct {
    Field string `flag:"flag-for-field" flag-usage:"description"`
}

fs := flag.NewFlagSet("myapp", flag.ExitOnError)

var conf Conf
if err := gonphig.ReadConfig(fs, &conf); err != nil {
    log.Fatal(err)
}

// Parse after ReadConfig — flags are registered by this point
if err := fs.Parse(os.Args[1:]); err != nil {
    log.Fatal(err)
}
```

For the standard case you can pass `flag.CommandLine` directly:

```go
if err := gonphig.ReadConfig(flag.CommandLine, &conf); err != nil {
    log.Fatal(err)
}
flag.Parse()
```

### Read configuration from ENV variables

Use the `env` tag to bind a field to an environment variable.

```go
type Conf struct {
    Field string `env:"ENV_VARIABLE"`
}

var conf Conf
if err := gonphig.ReadConfig(flag.CommandLine, &conf); err != nil {
    log.Fatal(err)
}
```

### Assign default values

Use the `default` tag to set a fallback value when neither a flag nor an env var is present.

```go
type Conf struct {
    Field   string        `env:"FIELD" default:"someValue"`
    Count   int           `env:"COUNT" default:"42"`
    Timeout time.Duration `env:"TIMEOUT" default:"30s"`
}
```

### Read configuration from a YAML file

YAML values are the lowest-priority source — they are overridden by env vars and flags.

```go
type Conf struct {
    Field string
}

var conf Conf
if err := gonphig.ReadFromFile("path/to/file.yml", flag.CommandLine, &conf); err != nil {
    log.Fatal(err)
}
flag.Parse()
```

Use the `yaml` tag to rename a field in the YAML file:

```go
type Conf struct {
    NotTheSameName string `yaml:"field"`
}
```

### Supported field types

| Go type         | env | flag | default |
|-----------------|-----|------|---------|
| `string`        | ✓   | ✓    | ✓       |
| `int`           | ✓   | ✓    | ✓       |
| `int64`         | ✓   | ✓    | ✓       |
| `float32`       | ✓   | ✓    | ✓       |
| `float64`       | ✓   | ✓    | ✓       |
| `bool`          | ✓   | ✓    | ✓       |
| `time.Duration` | ✓   | ✓    | ✓       |
| `[]string`      | ✓   | —    | ✓       |

`[]string` values are parsed as comma-separated: `HOST1,HOST2,HOST3`.

### Validation

Use the `validate` tag to enforce constraints on configuration values.

```go
type Conf struct {
    Field string `env:"ENV_VARIABLE" validate:"required"`
}
```

### Configuration hierarchy

Sources are applied in this order — higher entries win:

1. `flag` — CLI flags (after `fs.Parse` is called)
2. `env` — environment variables
3. `default` — tag default values
4. YAML file (when using `ReadFromFile`)

## External resources
### YAML
* [SPEC](https://yaml.org/spec/1.2/spec.html)
* [go-yaml](https://github.com/go-yaml/yaml)

### Validations
* [go-playground/validator](https://github.com/go-playground/validator)
