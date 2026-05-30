package gonphig

import (
	"flag"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configTestFile = "config-test.yml"
const configArraysTestFile = "config-arrays.yml"

type parentConfig struct {
	Field string `env:"string-env"`
	Child struct {
		Int   int `env:"int-env"`
		Child struct {
			Bool bool `env:"bool-env"`
		}
	}
}

type withFlagsConfig struct {
	Field string `flag:"string-flag"`
	Child struct {
		Int   int `flag:"int-flag"`
		Child struct {
			Bool bool `flag:"bool-flag"`
		}
	}
}

func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

// --- Core loading ---

func TestLoadFromFile(t *testing.T) {
	var config parentConfig
	err := Load(&config, WithFile(configTestFile))
	require.NoError(t, err)

	assert.Equal(t, "Hello", config.Field)
	assert.Equal(t, 1, config.Child.Int)
	assert.Equal(t, true, config.Child.Child.Bool)
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("string-env", "Bye!")
	t.Setenv("int-env", "100")
	t.Setenv("bool-env", "false")

	var config parentConfig
	err := Load(&config)
	require.NoError(t, err)

	assert.Equal(t, "Bye!", config.Field)
	assert.Equal(t, 100, config.Child.Int)
	assert.Equal(t, false, config.Child.Child.Bool)
}

func TestLoadEnvOverridesFile(t *testing.T) {
	t.Setenv("string-env", "Bye!")
	t.Setenv("int-env", "100")
	t.Setenv("bool-env", "false")

	var config parentConfig
	err := Load(&config, WithFile(configTestFile))
	require.NoError(t, err)

	assert.Equal(t, "Bye!", config.Field)
	assert.Equal(t, 100, config.Child.Int)
	assert.Equal(t, false, config.Child.Child.Bool)
}

func TestLoadFromFlags(t *testing.T) {
	fs := newFlagSet(t.Name())

	var config withFlagsConfig
	err := Load(&config,
		WithFile(configTestFile),
		WithFlags(fs, []string{"--string-flag=DUDE", "--int-flag=10000", "--bool-flag=true"}),
	)
	require.NoError(t, err)

	assert.Equal(t, "DUDE", config.Field)
	assert.Equal(t, 10000, config.Child.Int)
	assert.Equal(t, true, config.Child.Child.Bool)
}

func TestLoadFlagsOverrideFile(t *testing.T) {
	fs := newFlagSet(t.Name())

	var config withFlagsConfig
	err := Load(&config,
		WithFile(configTestFile),
		WithFlags(fs, []string{"--string-flag=OVERRIDE"}),
	)
	require.NoError(t, err)
	assert.Equal(t, "OVERRIDE", config.Field)
}

func TestLoadFlagDefaultFallsBackToFile(t *testing.T) {
	fs := newFlagSet(t.Name())

	var config withFlagsConfig
	err := Load(&config,
		WithFile(configTestFile),
		WithFlags(fs, []string{}),
	)
	require.NoError(t, err)
	// No flag passed — field keeps the YAML value as the flag default
	assert.Equal(t, "Hello", config.Field)
}

func TestWrongFlagTypeMessage(t *testing.T) {
	if os.Getenv("WRONG-FLAG-VALUE") == "1" {
		var config withFlagsConfig
		err := Load(&config,
			WithFile(configTestFile),
			WithFlags(newFlagSet(t.Name()), []string{"--int-flag=notAnInt"}),
		)
		if err != nil {
			os.Exit(1)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestWrongFlagTypeMessage")
	cmd.Env = append(os.Environ(), "WRONG-FLAG-VALUE=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatal("gonphig loading should have failed due to the wrong flag argument was used")
}

// --- Priority order ---

func TestLoadValueOrder(t *testing.T) {
	type testType struct {
		IntA int `default:"1"`
		IntB int `env:"intb-env" default:"1"`
		IntC int `env:"intb-env" flag:"intc-flag" default:"1"`
	}

	t.Setenv("intb-env", "2")

	var config testType
	err := Load(&config, WithFlags(newFlagSet(t.Name()), []string{"--intc-flag=3"}))
	require.NoError(t, err)

	assert.Equal(t, 1, config.IntA)  // default
	assert.Equal(t, 2, config.IntB)  // env wins over default
	assert.Equal(t, 3, config.IntC)  // flag wins over env
}

func TestLoadFileValuePreservedWhenNoOverride(t *testing.T) {
	type testType struct {
		Field string `yaml:"field"`
	}

	var config testType
	err := Load(&config, WithFile(configTestFile))
	require.NoError(t, err)
	assert.Equal(t, "Hello", config.Field)
}

func TestLoadDefaultDoesNotOverrideFile(t *testing.T) {
	type testType struct {
		Field string `yaml:"field" default:"fallback"`
	}

	var config testType
	err := Load(&config, WithFile(configTestFile))
	require.NoError(t, err)
	// YAML "Hello" wins over default "fallback"
	assert.Equal(t, "Hello", config.Field)
}

// --- Type support ---

func TestIntFamily(t *testing.T) {
	type testType struct {
		Int   int   `env:"int-env"`
		Int64 int64 `env:"int64-env"`
	}
	t.Setenv("int-env", "100")
	t.Setenv("int64-env", "10000")

	var config testType
	err := Load(&config)
	require.NoError(t, err)
	assert.Equal(t, 100, config.Int)
	assert.Equal(t, int64(10000), config.Int64)
}

func TestFloatFamily(t *testing.T) {
	type testType struct {
		Float32 float32 `env:"float32-env"`
		Float64 float64 `env:"float64-env"`
	}
	t.Setenv("float32-env", "100.01")
	t.Setenv("float64-env", "10000.01")

	var config testType
	err := Load(&config)
	require.NoError(t, err)
	assert.Equal(t, float32(100.01), config.Float32)
	assert.Equal(t, 10000.01, config.Float64)
}

func TestDefaultValues(t *testing.T) {
	type testType struct {
		Float32 float32       `env:"float32-def-env" default:"100.01"`
		Float64 float64       `env:"float64-def-env" default:"10000.01"`
		Int     int           `env:"int-env-def" default:"100"`
		Int64   int64         `env:"int64-env-def" default:"10000"`
		String  string        `env:"string-env-def" default:"something"`
		Bool    bool          `env:"bool-env-def" default:"true"`
		Timeout time.Duration `env:"timeout-def-env" default:"30s"`
	}

	var config testType
	err := Load(&config)
	require.NoError(t, err)

	assert.Equal(t, float32(100.01), config.Float32)
	assert.Equal(t, 10000.01, config.Float64)
	assert.Equal(t, 100, config.Int)
	assert.Equal(t, int64(10000), config.Int64)
	assert.Equal(t, "something", config.String)
	assert.True(t, config.Bool)
	assert.Equal(t, 30*time.Second, config.Timeout)

	t.Setenv("int-env-def", "1")
	var config2 testType
	err = Load(&config2)
	require.NoError(t, err)
	assert.Equal(t, 1, config2.Int)
	assert.Equal(t, "something", config2.String)
}

func TestFloat32FromFlag(t *testing.T) {
	type testType struct {
		Value float32 `flag:"float32-val" default:"1.5"`
	}

	var config testType
	err := Load(&config, WithFlags(newFlagSet(t.Name()), []string{"--float32-val=3.14"}))
	require.NoError(t, err)
	assert.InDelta(t, float32(3.14), config.Value, 0.001)
}

func TestFloat32DefaultFallback(t *testing.T) {
	type testType struct {
		Value float32 `flag:"float32-fallback" default:"2.5"`
	}

	var config testType
	err := Load(&config, WithFlags(newFlagSet(t.Name()), []string{}))
	require.NoError(t, err)
	assert.InDelta(t, float32(2.5), config.Value, 0.001)
}

// --- time.Duration ---

func TestDurationFromEnv(t *testing.T) {
	type testType struct {
		Timeout time.Duration `env:"TIMEOUT"`
	}

	t.Setenv("TIMEOUT", "5s")

	var config testType
	err := Load(&config)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, config.Timeout)
}

func TestDurationFromDefault(t *testing.T) {
	type testType struct {
		Timeout time.Duration `default:"30s"`
	}

	var config testType
	err := Load(&config)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestDurationFromFlag(t *testing.T) {
	type testType struct {
		Timeout time.Duration `flag:"timeout"`
	}

	var config testType
	err := Load(&config, WithFlags(newFlagSet(t.Name()), []string{"--timeout=10m"}))
	require.NoError(t, err)
	assert.Equal(t, 10*time.Minute, config.Timeout)
}

func TestDurationInvalidValueReturnsError(t *testing.T) {
	type testType struct {
		Timeout time.Duration `env:"TIMEOUT_BAD"`
	}

	t.Setenv("TIMEOUT_BAD", "not-a-duration")

	var config testType
	err := Load(&config)
	require.Error(t, err)
}

// --- []string ---

func TestStringSliceFromEnv(t *testing.T) {
	type testType struct {
		Hosts []string `env:"HOSTS"`
	}

	t.Setenv("HOSTS", "host1, host2, host3")

	var config testType
	err := Load(&config)
	require.NoError(t, err)
	assert.Equal(t, []string{"host1", "host2", "host3"}, config.Hosts)
}

func TestStringSliceFromDefault(t *testing.T) {
	type testType struct {
		Hosts []string `default:"a,b,c"`
	}

	var config testType
	err := Load(&config)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, config.Hosts)
}

func TestStringSliceEnvOverridesDefault(t *testing.T) {
	type testType struct {
		Hosts []string `env:"HOSTS2" default:"a,b"`
	}

	t.Setenv("HOSTS2", "x,y,z")

	var config testType
	err := Load(&config)
	require.NoError(t, err)
	assert.Equal(t, []string{"x", "y", "z"}, config.Hosts)
}

func TestStringSliceFromFile(t *testing.T) {
	type testType struct {
		Something []string
	}

	var config testType
	err := Load(&config, WithFile(configArraysTestFile))
	require.NoError(t, err)
	assert.Equal(t, 3, len(config.Something))
}

func TestStringSliceFileNotOverriddenByDefault(t *testing.T) {
	type testType struct {
		Something []string `default:"x,y"`
	}

	var config testType
	err := Load(&config, WithFile(configArraysTestFile))
	require.NoError(t, err)
	// YAML has 3 items — default should not override
	assert.Equal(t, 3, len(config.Something))
}

func TestStringSliceFlagTagReturnsError(t *testing.T) {
	type testType struct {
		Hosts []string `flag:"hosts"`
	}

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flag tag is not supported for slice fields")
}

// --- Validation ---

func TestRequiredFields(t *testing.T) {
	type testRequired struct {
		Field string `env:"string-req-env" validate:"required"`
	}

	var config testRequired
	err := Load(&config)
	require.Error(t, err)
	assert.Equal(t, "missing required configuration: Field", err.Error())

	t.Setenv("string-req-env", "Bye!")
	var ok testRequired
	err = Load(&ok)
	require.NoError(t, err)
	assert.Equal(t, "Bye!", ok.Field)
}

func TestRequiredIntFailsOnZero(t *testing.T) {
	type testType struct {
		Port int `validate:"required"`
	}

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Equal(t, "missing required configuration: Port", err.Error())
}

func TestRequiredBoolFailsWithUnsupportedError(t *testing.T) {
	type testType struct {
		Enabled bool `validate:"required"`
	}

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported on bool field")
}

func TestRequiredDurationFailsOnZero(t *testing.T) {
	type testType struct {
		Timeout time.Duration `validate:"required"`
	}

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Equal(t, "missing required configuration: Timeout", err.Error())
}

func TestRequiredOnNestedField(t *testing.T) {
	type testType struct {
		DB struct {
			Host string `validate:"required"`
		}
	}

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Equal(t, "missing required configuration: Host", err.Error())
}

func TestUnknownValidateRuleReturnsError(t *testing.T) {
	type testType struct {
		Field string `validate:"requried"`
	}

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown validation rule")
}

// --- WithArgs ---

func TestWithArgs(t *testing.T) {
	type testType struct {
		Host string `flag:"host" default:"localhost"`
		Port int    `flag:"port" default:"8080"`
	}

	var config testType
	err := Load(&config, WithArgs([]string{"--host=myserver", "--port=9090"}))
	require.NoError(t, err)
	assert.Equal(t, "myserver", config.Host)
	assert.Equal(t, 9090, config.Port)
}

func TestWithArgsDefaultFallback(t *testing.T) {
	type testType struct {
		Host string `flag:"host" default:"localhost"`
	}

	var config testType
	err := Load(&config, WithArgs([]string{}))
	require.NoError(t, err)
	assert.Equal(t, "localhost", config.Host)
}

// --- WithEnvPrefix ---

func TestWithEnvPrefix(t *testing.T) {
	type testType struct {
		Host string `env:"HOST"`
		Port int    `env:"PORT" default:"8080"`
	}

	t.Setenv("APP_HOST", "prefixed-host")
	t.Setenv("APP_PORT", "9090")

	var config testType
	err := Load(&config, WithEnvPrefix("APP"))
	require.NoError(t, err)
	assert.Equal(t, "prefixed-host", config.Host)
	assert.Equal(t, 9090, config.Port)
}

func TestWithEnvPrefixDoesNotMatchUnprefixed(t *testing.T) {
	type testType struct {
		Host string `env:"HOST" default:"fallback"`
	}

	t.Setenv("HOST", "unprefixed")

	var config testType
	err := Load(&config, WithEnvPrefix("APP"))
	require.NoError(t, err)
	// APP_HOST is not set, so default is used — unprefixed HOST is ignored
	assert.Equal(t, "fallback", config.Host)
}

func TestWithEnvPrefixUppercasesPrefix(t *testing.T) {
	type testType struct {
		Host string `env:"HOST"`
	}

	t.Setenv("APP_HOST", "ok")

	var config testType
	err := Load(&config, WithEnvPrefix("app"))
	require.NoError(t, err)
	assert.Equal(t, "ok", config.Host)
}

// --- Error messages include field name ---

func TestParseErrorIncludesFieldName(t *testing.T) {
	type testType struct {
		Port int `env:"BAD_PORT"`
	}

	t.Setenv("BAD_PORT", "not-an-int")

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Port")
}

func TestDurationParseErrorIncludesFieldName(t *testing.T) {
	type testType struct {
		Timeout time.Duration `env:"BAD_TIMEOUT"`
	}

	t.Setenv("BAD_TIMEOUT", "not-a-duration")

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Timeout")
}

// --- Bool + required ---

func TestRequiredBoolReturnsUnsupportedError(t *testing.T) {
	type testType struct {
		Enabled bool `validate:"required"`
	}

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported on bool field")
	assert.Contains(t, err.Error(), "Enabled")
}

// --- Input validation ---

func TestNilConfigReturnsError(t *testing.T) {
	err := Load(nil)
	require.Error(t, err)
	assert.Equal(t, "configuration must not be nil", err.Error())
}

func TestNonPointerStructReturnsError(t *testing.T) {
	type testType struct{ Field string }

	err := Load(testType{})
	require.Error(t, err)
	assert.Equal(t, "configuration to load needs to be a pointer", err.Error())
}

func TestInvalidTypeReturnsError(t *testing.T) {
	s := "not a struct"
	err := Load(&s)
	require.Error(t, err)
	assert.Equal(t, "invalid configuration structure", err.Error())
}

func TestUnsupportedFieldTypeReturnsError(t *testing.T) {
	type testType struct {
		Ch chan int
	}

	var config testType
	err := Load(&config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid field")
}

func TestWithFileMissingPathReturnsError(t *testing.T) {
	var config parentConfig
	err := Load(&config, WithFile("nonexistent.yml"))
	require.Error(t, err)
}

// --- Flag default fallback ---

func TestFlagDefaultFallback(t *testing.T) {
	type testType struct {
		Field string `flag:"field-with-default" default:"fallback"`
	}

	var config testType
	err := Load(&config, WithFlags(newFlagSet(t.Name()), []string{}))
	require.NoError(t, err)
	assert.Equal(t, "fallback", config.Field)
}
