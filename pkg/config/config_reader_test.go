package config

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configTestFile = "config-test"

type parentConfig struct {
	Field string `env:"string-env"`
	Child struct {
		IntField int `env:"int-env"`
		Child    struct {
			BoolField bool `env:"bool-env"`
		}
	}
}

type withFlagsConfig struct {
	Field string `flag:"string-flag"`
	Child struct {
		IntField int `flag:"int-flag"`
		Child    struct {
			BoolField bool `flag:"bool-flag"`
		}
	}
}

func TestReadConfigFromFile(t *testing.T) {
	var config parentConfig
	err := ReadConfiguration(configTestFile, &config)
	require.NoError(t, err)

	assert.Equal(t, "Hello", config.Field)
	assert.Equal(t, 1, config.Child.IntField)
	assert.Equal(t, true, config.Child.Child.BoolField)
}

func TestReadConfigFromEnvs(t *testing.T) {
	os.Setenv("string-env", "Bye!")
	os.Setenv("int-env", "100")
	os.Setenv("bool-env", "false")

	var config parentConfig
	err := ReadConfiguration(configTestFile, &config)
	require.NoError(t, err)

	assert.Equal(t, "Bye!", config.Field)
	assert.Equal(t, 100, config.Child.IntField)
	assert.Equal(t, false, config.Child.Child.BoolField)
}

func TestReadConfigFromFlags(t *testing.T) {
	os.Args = append(os.Args, "--string-flag=DUDE")
	os.Args = append(os.Args, "--int-flag=10000")
	os.Args = append(os.Args, "--bool-flag=true")

	var config withFlagsConfig
	err := ReadConfiguration(configTestFile, &config)
	require.NoError(t, err)

	assert.Equal(t, "DUDE", config.Field)
	assert.Equal(t, 10000, config.Child.IntField)
	assert.Equal(t, true, config.Child.Child.BoolField)
}

func TestWrongFlagTypeMessage(t *testing.T) {
	if os.Getenv("WRONG-FLAG-VALUE") == "1" {
		os.Args = append(os.Args, "--int-flag=notAnInt")
		var config withFlagsConfig
		_ = ReadConfiguration(configTestFile, &config)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestWrongFlagTypeMessage")
	cmd.Env = append(os.Environ(), "WRONG-FLAG-VALUE=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatal("config loading should have failed due to  the wrong flag argument was used")
}
