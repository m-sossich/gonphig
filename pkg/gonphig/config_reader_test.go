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

func TestReadConfigFromFile(t *testing.T) {
	var config parentConfig
	err := ReadFromFile(configTestFile, newFlagSet(t.Name()), &config)
	require.NoError(t, err)

	assert.Equal(t, "Hello", config.Field)
	assert.Equal(t, 1, config.Child.Int)
	assert.Equal(t, true, config.Child.Child.Bool)
}

func TestReadConfigFromEnvs(t *testing.T) {
	t.Setenv("string-env", "Bye!")
	t.Setenv("int-env", "100")
	t.Setenv("bool-env", "false")

	var config parentConfig
	err := ReadFromFile(configTestFile, newFlagSet(t.Name()), &config)
	require.NoError(t, err)

	assert.Equal(t, "Bye!", config.Field)
	assert.Equal(t, 100, config.Child.Int)
	assert.Equal(t, false, config.Child.Child.Bool)
}

func TestReadConfigFromFlags(t *testing.T) {
	fs := newFlagSet(t.Name())

	var config withFlagsConfig
	err := ReadFromFile(configTestFile, fs, &config)
	require.NoError(t, err)

	require.NoError(t, fs.Parse([]string{"--string-flag=DUDE", "--int-flag=10000", "--bool-flag=true"}))

	assert.Equal(t, "DUDE", config.Field)
	assert.Equal(t, 10000, config.Child.Int)
	assert.Equal(t, true, config.Child.Child.Bool)
}

func TestWrongFlagTypeMessage(t *testing.T) {
	if os.Getenv("WRONG-FLAG-VALUE") == "1" {
		fs := newFlagSet(t.Name())
		var config withFlagsConfig
		_ = ReadFromFile(configTestFile, fs, &config)
		if err := fs.Parse([]string{"--int-flag=notAnInt"}); err != nil {
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

func TestReadConfig(t *testing.T) {
	t.Setenv("string-env", "Bye!")
	t.Setenv("int-env", "100")
	t.Setenv("bool-env", "false")

	var config parentConfig
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.NoError(t, err)

	assert.Equal(t, "Bye!", config.Field)
	assert.Equal(t, 100, config.Child.Int)
	assert.Equal(t, false, config.Child.Child.Bool)
}

func TestRequiredFields(t *testing.T) {
	type testRequired struct {
		Field string `env:"string-req-env" validate:"required"`
	}

	var config testRequired
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.Error(t, err)
	assert.Equal(t, "missing required configuration: Field", err.Error())

	t.Setenv("string-req-env", "Bye!")
	var ok testRequired
	err = ReadConfig(newFlagSet(t.Name()), &ok)
	require.NoError(t, err)
	assert.Equal(t, "Bye!", ok.Field)

	type testNotRequired struct {
		Field string `env:"other-env"`
	}

	var notRequired testNotRequired
	err = ReadConfig(newFlagSet(t.Name()), &notRequired)
	require.NoError(t, err)
	assert.Equal(t, "", notRequired.Field)
}

func TestIntFamily(t *testing.T) {
	type testType struct {
		Int   int   `env:"int-env"`
		Int64 int64 `env:"int64-env"`
	}
	t.Setenv("int-env", "100")
	t.Setenv("int64-env", "10000")

	var config testType
	err := ReadConfig(newFlagSet(t.Name()), &config)
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
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.NoError(t, err)
	assert.Equal(t, float32(100.01), config.Float32)
	assert.Equal(t, 10000.01, config.Float64)
}

func TestDefaultValues(t *testing.T) {
	type testType struct {
		Float32 float32 `env:"float32-def-env" default:"100.01"`
		Float64 float64 `env:"float64-def-env" default:"10000.01"`
		Int     int     `env:"int-env-def" default:"100"`
		Int64   int64   `env:"int64-env-def" default:"10000"`
		String  string  `env:"string-env-def" default:"something"`
		Bool    bool    `env:"bool-env-def" default:"true"`
	}
	var config testType
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.NoError(t, err)

	assert.Equal(t, float32(100.01), config.Float32)
	assert.Equal(t, 10000.01, config.Float64)
	assert.Equal(t, 100, config.Int)
	assert.Equal(t, int64(10000), config.Int64)
	assert.Equal(t, "something", config.String)
	assert.True(t, config.Bool)

	t.Setenv("int-env-def", "1")
	var config2 testType
	err = ReadConfig(newFlagSet(t.Name()), &config2)
	require.NoError(t, err)

	assert.Equal(t, float32(100.01), config2.Float32)
	assert.Equal(t, 10000.01, config2.Float64)
	assert.Equal(t, 1, config2.Int)
	assert.Equal(t, int64(10000), config2.Int64)
	assert.Equal(t, "something", config2.String)
	assert.True(t, config2.Bool)
}

func TestReadValueOrder(t *testing.T) {
	type testType struct {
		IntA int `default:"1"`
		IntB int `env:"intb-env" default:"1"`
		IntC int `env:"intb-env" flag:"intc-flag" default:"1"`
	}

	t.Setenv("intb-env", "2")

	fs := newFlagSet(t.Name())
	var config testType
	err := ReadConfig(fs, &config)
	require.NoError(t, err)

	require.NoError(t, fs.Parse([]string{"--intc-flag=3"}))

	assert.Equal(t, 1, config.IntA)
	assert.Equal(t, 2, config.IntB)
	assert.Equal(t, 3, config.IntC)
}

func TestReadArraysFromConfigFromFile(t *testing.T) {
	type testType struct {
		Something []string
	}

	var config testType
	err := ReadFromFile(configArraysTestFile, newFlagSet(t.Name()), &config)
	require.NoError(t, err)

	assert.Equal(t, 3, len(config.Something))
}

func TestStringSliceFromEnv(t *testing.T) {
	type testType struct {
		Hosts []string `env:"HOSTS"`
	}

	t.Setenv("HOSTS", "host1, host2, host3")

	var config testType
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.NoError(t, err)
	assert.Equal(t, []string{"host1", "host2", "host3"}, config.Hosts)
}

func TestStringSliceFromDefault(t *testing.T) {
	type testType struct {
		Hosts []string `default:"a,b,c"`
	}

	var config testType
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, config.Hosts)
}

func TestStringSliceEnvOverridesDefault(t *testing.T) {
	type testType struct {
		Hosts []string `env:"HOSTS2" default:"a,b"`
	}

	t.Setenv("HOSTS2", "x,y,z")

	var config testType
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.NoError(t, err)
	assert.Equal(t, []string{"x", "y", "z"}, config.Hosts)
}

func TestDurationFromEnv(t *testing.T) {
	type testType struct {
		Timeout time.Duration `env:"TIMEOUT"`
	}

	t.Setenv("TIMEOUT", "5s")

	var config testType
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, config.Timeout)
}

func TestDurationFromDefault(t *testing.T) {
	type testType struct {
		Timeout time.Duration `default:"30s"`
	}

	var config testType
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestDurationFromFlag(t *testing.T) {
	type testType struct {
		Timeout time.Duration `flag:"timeout"`
	}

	fs := newFlagSet(t.Name())
	var config testType
	err := ReadConfig(fs, &config)
	require.NoError(t, err)

	require.NoError(t, fs.Parse([]string{"--timeout=10m"}))
	assert.Equal(t, 10*time.Minute, config.Timeout)
}

func TestDurationInvalidValueReturnsError(t *testing.T) {
	type testType struct {
		Timeout time.Duration `env:"TIMEOUT_BAD"`
	}

	t.Setenv("TIMEOUT_BAD", "not-a-duration")

	var config testType
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.Error(t, err)
}

func TestStringSliceFlagTagReturnsError(t *testing.T) {
	type testType struct {
		Hosts []string `flag:"hosts"`
	}

	var config testType
	err := ReadConfig(newFlagSet(t.Name()), &config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flag tag is not supported for slice fields")
}
