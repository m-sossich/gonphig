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