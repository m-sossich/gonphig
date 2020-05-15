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
	readEnvKey  = "env"
	readFlagKey = "flag"
	defaultKey  = "default"
	flagUsage   = "flag-usage"
)

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
		return errors.New("configuration to load need to be a pointer")
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
		if err := overwriteValue(f, v, setInt64); err != nil {
			return err
		}

	case reflect.Int:
		if err := overwriteValue(f, v, setInt); err != nil {
			return err
		}

	case reflect.Float32, reflect.Float64:
		if err := overwriteValue(f, v, setFloat64); err != nil {
			return err
		}

	case reflect.String:
		if err := overwriteValue(f, v, setString); err != nil {
			return err
		}

	case reflect.Bool:
		if err := overwriteValue(f, v, setBool); err != nil {
			return err
		}

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
	val, ok := t.Lookup(readFlagKey)
	if ok {
		flag.StringVar(v.Addr().Interface().(*string), val, v.String(), getUsage(t))
		return nil
	}
	val, ok = t.Lookup(readEnvKey)
	if ok {
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
	val, ok := t.Lookup(readFlagKey)
	if ok {
		flag.BoolVar(v.Addr().Interface().(*bool), val, v.Bool(), getUsage(t))
		return nil
	}
	val, ok = t.Lookup(readEnvKey)
	if ok {
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
	val, ok := t.Lookup(readFlagKey)
	if ok {
		flag.Int64Var(v.Addr().Interface().(*int64), val, v.Int(), getUsage(t))
		return nil
	}
	val, ok = t.Lookup(readEnvKey)
	if ok {
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
	val, ok := t.Lookup(readFlagKey)
	if ok {
		flag.IntVar(v.Addr().Interface().(*int), val, int(v.Int()), getUsage(t))
		return nil
	}
	val, ok = t.Lookup(readEnvKey)
	if ok {
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
	val, ok := t.Lookup(readFlagKey)
	if ok {
		flag.Float64Var(v.Addr().Interface().(*float64), val, v.Float(), getUsage(t))
		return nil
	}
	val, ok = t.Lookup(readEnvKey)
	if ok {
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
