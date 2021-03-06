package gonphig

import (
	"errors"
	"flag"
	"fmt"
	"github.com/m-sossich/gonphig/internal/validation"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const (
	// Use the 'env' tag to mark than an attribute might be overwritten if such env-var is set
	readEnvKey = "env"
	// Use the 'flag' tag to mark than an attribute might be overwritten if such flag is set
	readFlagKey = "flag"
	// Use the 'default' tag set a default value for an attribute
	defaultKey = "default"
	// Use the 'flag-usage' to add a description for the expected flag. This is optional
	flagUsage = "flag-usage"
)

// ReadFromFile loads configurations into the config struct provided.
// Default values from the given yaml file. If indicated, the values might be overwritten by a env-var or flag
func ReadFromFile(configPath string, c interface{}) error {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(configFile, c)
	if err != nil {
		return err
	}
	return ReadConfig(c)
}

// ReadConfig loads configurations into the config struct provided. Default should be provided using the 'default' tag.
// If indicated, the values might be overwritten by a env-var or flag
func ReadConfig(c interface{}) error {
	t := reflect.TypeOf(c)

	v, err := validation.WithMessages(map[string]string{"required": "missing required configuration: {0}"})
	if err != nil {
		return err
	}

	switch t.Kind() {
	case reflect.Ptr, reflect.Interface:
		val := t.Elem()
		fields := val.NumField()
		for i := 0; i < fields; i++ {
			value := reflect.ValueOf(c).Elem().Field(i)
			field := val.Field(i)
			if err := overwriteFields(field, &value); err != nil {
				return err
			}
		}

		flag.Parse()

		return v.ValidateStruct(c)
	case reflect.Struct:
		return errors.New("configuration to load needs to be a pointer")
	default:
		return errors.New("invalid configuration structure")
	}
}

func overwriteFields(f reflect.StructField, v *reflect.Value) error {
	switch f.Type.Kind() {
	case reflect.Struct:
		t := f.Type
		fields := t.NumField()
		for i := 0; i < fields; i++ {
			value := v.Field(i)
			if err := overwriteFields(t.Field(i), &value); err != nil {
				return err
			}
		}

	case reflect.Int64:
		return overwriteValue(f, v, setInt64)

	case reflect.Int:
		return overwriteValue(f, v, setInt)

	case reflect.Float32, reflect.Float64:
		return overwriteValue(f, v, setFloat64)

	case reflect.String:
		return overwriteValue(f, v, setString)

	case reflect.Bool:
		return overwriteValue(f, v, setBool)

	case reflect.Slice, reflect.Map:
		return overwriteValue(f, v, identity)

	default:
		return fmt.Errorf("invalid field[%s] type[%s]", f.Name, f.Type.Name())
	}
	return nil
}

func overwriteValue(f reflect.StructField, v *reflect.Value, setValue func(v *reflect.Value, t reflect.StructTag) error) error {
	tag := f.Tag
	if len(tag) > 0 {
		return setValue(v, tag)
	}
	return nil
}

func setString(v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		flag.StringVar(v.Addr().Interface().(*string), val, v.String(), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		variable := os.Getenv(val)
		if len(variable) > 0 {
			v.SetString(os.Getenv(val))
			return nil
		}
	}
	if def, ok := t.Lookup(defaultKey); ok && def != "" {
		v.SetString(strings.TrimSpace(def))
	}
	return nil
}

func setBool(v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		flag.BoolVar(v.Addr().Interface().(*bool), val, v.Bool(), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		value := os.Getenv(val)
		if len(value) > 0 {
			return parseBool(v, value)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseBool(v, strings.ToLower(def))
	}
	return nil
}

func parseBool(v *reflect.Value, val string) error {
	trimmed := strings.TrimSpace(val)
	if len(trimmed) > 0 {
		parsed, err := strconv.ParseBool(trimmed)
		if err != nil {
			return err
		}
		v.SetBool(parsed)
	}
	return nil
}

func setInt64(v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		flag.Int64Var(v.Addr().Interface().(*int64), val, v.Int(), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		value := os.Getenv(val)
		if len(value) > 0 {
			return parseInt64(v, value)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseInt64(v, def)
	}
	return nil
}

func setInt(v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		flag.IntVar(v.Addr().Interface().(*int), val, int(v.Int()), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		value := os.Getenv(val)
		if len(value) > 0 {
			return parseInt64(v, value)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseInt64(v, def)
	}
	return nil
}

func parseInt64(v *reflect.Value, val string) error {
	trimmed := strings.TrimSpace(val)
	if len(trimmed) > 0 {
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(parsed)
	}
	return nil
}

func setFloat64(v *reflect.Value, t reflect.StructTag) error {
	if val, ok := t.Lookup(readFlagKey); ok {
		flag.Float64Var(v.Addr().Interface().(*float64), val, v.Float(), getUsage(t))
		return nil
	}
	if val, ok := t.Lookup(readEnvKey); ok {
		value := os.Getenv(val)
		if len(value) > 0 {
			return parseFloat64(v, value)
		}
	}
	if def, ok := t.Lookup(defaultKey); ok {
		return parseFloat64(v, def)
	}
	return nil
}

func identity(v *reflect.Value, t reflect.StructTag) error {
	return nil
}

func parseFloat64(v *reflect.Value, val string) error {
	trimmed := strings.TrimSpace(val)
	if len(trimmed) > 0 {
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return err
		}
		v.SetFloat(parsed)
	}
	return nil
}

func getUsage(tag reflect.StructTag) string {
	val, ok := tag.Lookup(flagUsage)
	if ok {
		return val
	}
	return ""
}
