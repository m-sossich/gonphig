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
)

const (
	readEnvKey  = "env"
	readFlagKey = "flag"
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
	val, ok := t.Lookup(readEnvKey)
	if ok {
		variable := os.Getenv(val)
		if len(variable) > 0 {
			v.SetString(os.Getenv(val))
		}
	}
	val, ok = t.Lookup(readFlagKey)
	if ok {
		flag.StringVar(v.Addr().Interface().(*string), val, v.String(), getUsage(t))
	}
	return nil
}

func setBool(v *reflect.Value, t reflect.StructTag) error {
	val, ok := t.Lookup(readEnvKey)
	if ok {
		variable := os.Getenv(val)
		if len(variable) > 0 {
			parsed, err := strconv.ParseBool(os.Getenv(val))
			if err != nil {
				return err
			}
			v.SetBool(parsed)
		}
	}
	val, ok = t.Lookup(readFlagKey)
	if ok {
		flag.BoolVar(v.Addr().Interface().(*bool), val, v.Bool(), getUsage(t))
	}
	return nil
}

func setInt64(v *reflect.Value, t reflect.StructTag) error {
	val, ok := t.Lookup(readEnvKey)
	if ok {
		variable := os.Getenv(val)
		if len(variable) > 0 {
			parsed, err := strconv.ParseInt(os.Getenv(val), 10, 64)
			if err != nil {
				return err
			}
			v.SetInt(parsed)
		}
	}
	val, ok = t.Lookup(readFlagKey)
	if ok {
		flag.Int64Var(v.Addr().Interface().(*int64), val, v.Int(), getUsage(t))
	}
	return nil
}

func setInt(v *reflect.Value, t reflect.StructTag) error {
	val, ok := t.Lookup(readEnvKey)
	if ok {
		variable := os.Getenv(val)
		if len(variable) > 0 {
			parsed, err := strconv.ParseInt(os.Getenv(val), 10, 64)
			if err != nil {
				return err
			}
			v.SetInt(parsed)
		}
	}
	val, ok = t.Lookup(readFlagKey)
	if ok {
		flag.IntVar(v.Addr().Interface().(*int), val, int(v.Int()), getUsage(t))
	}
	return nil
}

func setFloat64(v *reflect.Value, t reflect.StructTag) error {
	val, ok := t.Lookup(readEnvKey)
	if ok {
		variable := os.Getenv(val)
		if len(variable) > 0 {
			parsed, err := strconv.ParseFloat(os.Getenv(val), 64)
			if err != nil {
				return err
			}
			v.SetFloat(parsed)
		}
	}
	val, ok = t.Lookup(readFlagKey)
	if ok {
		flag.Float64Var(v.Addr().Interface().(*float64), val, v.Float(), getUsage(t))
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
