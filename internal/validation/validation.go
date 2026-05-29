// Package validation provides struct validation for gonphig configuration
// structs. It is intentionally minimal: the only supported rule is "required",
// which checks that a field is not the zero value for its type.
package validation

import (
	"fmt"
	"reflect"
	"strings"
)

// ValidateRequired inspects c for fields tagged validate:"required" and
// returns an error for the first field whose value is the zero value for its
// type (e.g. "" for string, 0 for numeric types, false for bool).
//
// Unknown rules in the validate tag (e.g. validate:"min=1") return an error
// immediately so typos fail loudly rather than silently skipping validation.
//
// c must be a non-nil pointer to a struct. ValidateRequired recurses into
// nested structs automatically.
//
// Error format: "missing required configuration: <FieldName>"
func ValidateRequired(c any) error {
	return walk(reflect.TypeOf(c).Elem(), reflect.ValueOf(c).Elem())
}

// walk traverses t/v field-by-field, recursing into nested structs, and
// checks the validate tag on each non-struct field.
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
			switch strings.TrimSpace(rule) {
			case "required":
				if value.IsZero() {
					return fmt.Errorf("missing required configuration: %s", field.Name)
				}
			case "":
				// ignore empty segments from trailing commas
			default:
				return fmt.Errorf("unknown validation rule %q on field %s", strings.TrimSpace(rule), field.Name)
			}
		}
	}
	return nil
}
