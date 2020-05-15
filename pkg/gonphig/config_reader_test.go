package gonphig

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configTestFile = "config-test.yml"

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

func TestReadConfigFromFile(t *testing.T) {
	var config parentConfig
	err := ReadFromFile(configTestFile, &config)
	require.NoError(t, err)

	assert.Equal(t, "Hello", config.Field)
	assert.Equal(t, 1, config.Child.Int)
	assert.Equal(t, true, config.Child.Child.Bool)
}

func TestReadConfigFromEnvs(t *testing.T) {
	os.Setenv("string-env", "Bye!")
	os.Setenv("int-env", "100")
	os.Setenv("bool-env", "false")

	var config parentConfig
	err := ReadFromFile(configTestFile, &config)
	require.NoError(t, err)

	assert.Equal(t, "Bye!", config.Field)
	assert.Equal(t, 100, config.Child.Int)
	assert.Equal(t, false, config.Child.Child.Bool)
}

func TestReadConfigFromFlags(t *testing.T) {
	os.Args = append(os.Args, "--string-flag=DUDE")
	os.Args = append(os.Args, "--int-flag=10000")
	os.Args = append(os.Args, "--bool-flag=true")

	var config withFlagsConfig
	err := ReadFromFile(configTestFile, &config)
	require.NoError(t, err)

	assert.Equal(t, "DUDE", config.Field)
	assert.Equal(t, 10000, config.Child.Int)
	assert.Equal(t, true, config.Child.Child.Bool)
}

func TestWrongFlagTypeMessage(t *testing.T) {
	if os.Getenv("WRONG-FLAG-VALUE") == "1" {
		os.Args = append(os.Args, "--int-flag=notAnInt")
		var config withFlagsConfig
		_ = ReadFromFile(configTestFile, &config)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestWrongFlagTypeMessage")
	cmd.Env = append(os.Environ(), "WRONG-FLAG-VALUE=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatal("gonphig loading should have failed due to  the wrong flag argument was used")
}

func TestReadConfig(t *testing.T) {
	os.Setenv("string-env", "Bye!")
	os.Setenv("int-env", "100")
	os.Setenv("bool-env", "false")

	var config parentConfig
	err := ReadConfig(&config)
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
	err := ReadConfig(&config)
	require.Error(t, err)
	assert.Equal(t, "missing required configuration: Field", err.Error())

	os.Setenv("string-req-env", "Bye!")
	var ok testRequired
	err = ReadConfig(&ok)

	assert.Equal(t, "Bye!", ok.Field)

	type testNotRequired struct {
		Field string `env:"other-env"`
	}

	var notRequired testNotRequired
	err = ReadConfig(&notRequired)
	require.NoError(t, err)
	assert.Equal(t, "", notRequired.Field)
}

func TestIntFamily(t *testing.T) {
	type testType struct {
		Int   int   `env:"int-env"`
		Int64 int64 `env:"int64-env"`
	}
	os.Setenv("int-env", "100")
	os.Setenv("int64-env", "10000")

	var config testType
	err := ReadConfig(&config)
	require.NoError(t, err)
	assert.Equal(t, 100, config.Int)
	assert.Equal(t, int64(10000), config.Int64)
}

func TestFloatFamily(t *testing.T) {
	type testType struct {
		Float32 float32 `env:"float32-env"`
		Float64 float64 `env:"float64-env"`
	}
	os.Setenv("float32-env", "100.01")
	os.Setenv("float64-env", "10000.01")

	var config testType
	err := ReadConfig(&config)
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
	err := ReadConfig(&config)

	require.NoError(t, err)

	assert.Equal(t, float32(100.01), config.Float32)
	assert.Equal(t, 10000.01, config.Float64)
	assert.Equal(t, 100, config.Int)
	assert.Equal(t, int64(10000), config.Int64)
	assert.Equal(t, "something", config.String)
	assert.True(t, config.Bool)

	//Env variables overwrite default value
	os.Setenv("int-env-def", "1")
	var config2 testType
	err = ReadConfig(&config2)
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

	os.Setenv("intb-env", "2")
	os.Args = append(os.Args, "--intc-flag=3")

	var config testType
	err := ReadConfig(&config)
	require.NoError(t, err)

	assert.Equal(t, 1, config.IntA)
	assert.Equal(t, 2, config.IntB)
	assert.Equal(t, 3, config.IntC)
}
