package validation

import (
	"errors"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"gopkg.in/go-playground/validator.v9"
	enTranslations "gopkg.in/go-playground/validator.v9/translations/en"
)

type Validator interface {
	ValidateStruct(s interface{}) error
}

type validatorImpl struct {
	validator    *validator.Validate
	translations ut.Translator
}

const (
	requiredErrorMessage = "{0} field is required."
)

func DefaultValidator() (Validator, error) {
	m := map[string]string{}
	m["required"] = requiredErrorMessage
	return WithMessages(m)
}

func WithMessages(translations map[string]string) (Validator, error) {
	v := validator.New()
	t, err := setupMessages(v, translations)
	if err != nil {
		return nil, err
	}
	return &validatorImpl{v, t}, nil
}

func (v *validatorImpl) ValidateStruct(s interface{}) error {
	err := v.validator.Struct(s)
	if err != nil {
		validationErrors := err.(validator.ValidationErrors)
		if validationErrors != nil {
			return translateErrors(validationErrors, v.translations)
		}
	}
	return nil
}

func setupMessages(v *validator.Validate, translations map[string]string) (ut.Translator, error) {
	eng := en.New()
	uni := ut.New(eng, eng)
	trans, _ := uni.GetTranslator("en")

	for tag, msg := range translations {
		if err := addTranslation(tag, msg, trans, v); err != nil {
			return nil, err
		}
	}

	enTranslations.RegisterDefaultTranslations(v, trans)

	return trans, nil
}

func addTranslation(tag string, messageTemplate string, trans ut.Translator, v *validator.Validate) error {
	return v.RegisterTranslation(tag, trans, func(ut ut.Translator) error {
		return ut.Add(tag, messageTemplate, true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T(tag, fe.Field(), fe.Param())
		return t
	})
}

func translateErrors(errs []validator.FieldError, t ut.Translator) error {
	translations := []string{}
	for _, e := range errs {
		translations = append(translations, e.Translate(t))
	}
	err := errors.New(strings.Join(translations, ", "))
	return err
}
