package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configTestFile = "config-test"

type parentConfig struct {
	Field     string `overwrite-env:"string-env"`
	FieldFlag string `overwrite-arg:"string-flag"`
	Child     childConfig
}

type childConfig struct {
	IntField     int `overwrite-env:"int-env"`
	IntFlagField int `overwrite-arg:"int-flag"`
	Child        thirdLevelConfing
}

type thirdLevelConfing struct {
	BoolField     bool `overwrite-env:"bool-env"`
	BoolFlagField bool `overwrite-arg:"bool-flag"`
}

func TestReadConfigFromFile(t *testing.T) {
	var config parentConfig
	err := ReadConfiguration(configTestFile, &config)
	require.NoError(t, err)

	assert.Equal(t, "Hello", config.Field)
	assert.Equal(t, "", config.FieldFlag)

	assert.Equal(t, 1, config.Child.IntField)
	assert.Equal(t, 0, config.Child.IntFlagField)

	assert.Equal(t, true, config.Child.Child.BoolField)
	assert.Equal(t, false, config.Child.Child.BoolFlagField)

	os.Setenv("string-env", "Bye!")
	os.Setenv("int-env", "100")
	os.Setenv("bool-env", "false")

	var envConfigs parentConfig
	err = ReadConfiguration(configTestFile, &envConfigs)
	require.NoError(t, err)

	assert.Equal(t, "Bye!", envConfigs.Field)
	assert.Equal(t, 100, envConfigs.Child.IntField)
	assert.Equal(t, false, envConfigs.Child.Child.BoolField)

}
