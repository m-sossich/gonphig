package config

import (
	"errors"
	"flag"
	"fmt"
	"github.com/m-sossich/gonphig/internal/validation"
	"reflect"

	"github.com/spf13/viper"
)

const (
	overwriteEnvKey  = "overwrite-env"
	overwriteFlagKey = "overwrite-flag"
	flagUsage        = "flag-usage"
)

func ReadConfiguration(configPath string, c interface{}) error {
	t := reflect.TypeOf(c)

	v, err := validation.WithMessages(map[string]string{"required": "missing required configuration: {0}"})
	if err != nil {
		return err
	}

	switch t.Kind() {
	case reflect.Ptr, reflect.Interface:
		viper.SetConfigName(configPath)
		viper.AddConfigPath(".")

		err := viper.ReadInConfig()
		if err != nil {
			return fmt.Errorf("error reading config file, %s", err)
		}

		val := t.Elem()
		fields := val.NumField()
		for i := 0; i < fields; i++ {
			child := val.Field(i)
			if err := overwriteFields("", child, viper.GetViper()); err != nil {
				return err
			}
		}

		flag.Parse()

		if err := viper.Unmarshal(c); err != nil {
			return err
		}

		return v.ValidateStruct(c)
	case reflect.Struct:
		return errors.New("configuration to load need to be a pointer")
	default:
		return errors.New("invalid configuration structure")

	}
}

func overwriteFields(parent string, f reflect.StructField, v *viper.Viper) error {
	prefix := ""
	if len(parent) > 0 {
		prefix = parent + "."
	}

	switch f.Type.Kind() {
	case reflect.Struct:
		t := f.Type
		fields := t.NumField()
		for i := 0; i < fields; i++ {
			child := t.Field(i)
			if err := overwriteFields(prefix+f.Name, child, v); err != nil {
				return err
			}
		}
	case reflect.Int:
		if err := overwriteValue(prefix, f, v, setIntFlag); err != nil {
			return err
		}
	case reflect.Int64:
		if err := overwriteValue(prefix, f, v, setInt64Flag); err != nil {
			return err
		}
	case reflect.String:
		if err := overwriteValue(prefix, f, v, setStringFlag); err != nil {
			return err
		}
	case reflect.Bool:
		if err := overwriteValue(prefix, f, v, setBoolFlag); err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid field[%s] type[%s]", f.Name, f.Type.Name())
	}
	return nil
}

func overwriteValue(prefix string, f reflect.StructField, v *viper.Viper, setFlag func(v *viper.Viper, flagVal string, key string, usage string)) error {
	tag := f.Tag
	if len(tag) > 0 {
		val, ok := tag.Lookup(overwriteEnvKey)
		if ok {
			err := v.BindEnv(prefix+f.Name, val)
			if err != nil {
				return err
			}
		}
		val, ok = tag.Lookup(overwriteFlagKey)
		if ok {
			key := prefix + f.Name
			setFlag(v, val, key, getUsage(tag))
		}
	}
	return nil
}

func setStringFlag(v *viper.Viper, flagVal string, key string, usage string) {
	viper.Set(key, flag.String(flagVal, viper.GetString(key), usage))
}

func setBoolFlag(v *viper.Viper, flagVal string, key string, usage string) {
	viper.Set(key, flag.Bool(flagVal, viper.GetBool(key), usage))
}

func setIntFlag(v *viper.Viper, flagVal string, key string, usage string) {
	viper.Set(key, flag.Int(flagVal, viper.GetInt(key), usage))
}

func setInt64Flag(v *viper.Viper, flagVal string, key string, usage string) {
	viper.Set(key, flag.Int64(flagVal, viper.GetInt64(key), usage))
}

func getUsage(tag reflect.StructTag) string {
	val, ok := tag.Lookup(flagUsage)
	if ok {
		return val
	}
	return ""
}
