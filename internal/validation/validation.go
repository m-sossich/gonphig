package validation

import (
	"fmt"
	"reflect"
	"strings"
)

// ValidateRequired walks the struct and returns an error for any field tagged
// validate:"required" whose value is the zero value for its type.
func ValidateRequired(c interface{}) error {
	return walk(reflect.TypeOf(c).Elem(), reflect.ValueOf(c).Elem())
}

func walk(t reflect.Type, v reflect.Value) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		if field.Type.Kind() == reflect.Struct {
			if err := walk(field.Type, value); err != nil {
				return err
			}
			continue
		}

		tag, ok := field.Tag.Lookup("validate")
		if !ok {
			continue
		}

		for _, rule := range strings.Split(tag, ",") {
			if strings.TrimSpace(rule) == "required" && value.IsZero() {
				return fmt.Errorf("missing required configuration: %s", field.Name)
			}
		}
	}
	return nil
}
