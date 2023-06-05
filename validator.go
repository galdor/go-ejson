package ejson

import (
	"bytes"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
)

type ValidationError struct {
	Pointer Pointer `json:"pointer"`
	Code    string  `json:"code"`
	Message string  `json:"message"`
}

type ValidationErrors []*ValidationError

type Validator struct {
	Pointer Pointer
	Errors  ValidationErrors
}

type Validatable interface {
	ValidateJSON(v *Validator)
}

func (err ValidationError) Error() string {
	if len(err.Pointer) == 0 {
		return err.Message
	} else {
		return fmt.Sprintf("%v: %s", err.Pointer, err.Message)
	}
}

func (errs ValidationErrors) Error() string {
	var buf bytes.Buffer

	buf.WriteString("invalid data:")

	for _, err := range errs {
		buf.WriteString("\n  ")
		buf.WriteString(err.Error())
	}

	return buf.String()
}

func Validate(value interface{}) error {
	v := NewValidator()

	if validatableValue, ok := value.(Validatable); ok {
		validatableValue.ValidateJSON(v)
	}

	if len(v.Errors) > 0 {
		return v.Error()
	}

	return nil
}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) Error() error {
	if len(v.Errors) == 0 {
		return nil
	}

	return v.Errors
}

func (v *Validator) Push(token interface{}) {
	v.Pointer = v.Pointer.Child(token)
}

func (v *Validator) Pop() {
	v.Pointer = v.Pointer.Parent()
}

func (v *Validator) WithChild(token interface{}, fn func()) {
	v.Push(token)
	defer v.Pop()

	fn()
}

func (v *Validator) AddError(token interface{}, code, format string, args ...interface{}) {
	pointer := v.Pointer.Child(token)

	err := ValidationError{
		Pointer: pointer,
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}

	v.Errors = append(v.Errors, &err)
}

func (v *Validator) Check(token interface{}, value bool, code, format string, args ...interface{}) bool {
	if !value {
		v.AddError(token, code, format, args...)
	}

	return value
}

func (v *Validator) CheckIntMin(token interface{}, i int, min int) bool {
	return v.Check(token, i >= min, "integer_too_small",
		"integer must be greater or equal to %d", min)
}

func (v *Validator) CheckIntMax(token interface{}, i int, max int) bool {
	return v.Check(token, i <= max, "integer_too_large",
		"integer must be lower or equal to %d", max)
}

func (v *Validator) CheckIntMinMax(token interface{}, i int, min, max int) bool {
	if !v.CheckIntMin(token, i, min) {
		return false
	}

	return v.CheckIntMax(token, i, max)
}

func (v *Validator) CheckFloatMin(token interface{}, i, min float64) bool {
	return v.Check(token, i >= min, "float_too_small",
		"float %f must be greater or equal to %f", i, min)
}

func (v *Validator) CheckFloatMax(token interface{}, i, max float64) bool {
	return v.Check(token, i <= max, "float_too_large",
		"float %f must be lower or equal to %f", i, max)
}

func (v *Validator) CheckFloatMinMax(token interface{}, i, min, max float64) bool {
	if !v.CheckFloatMin(token, i, min) {
		return false
	}

	return v.CheckFloatMax(token, i, max)
}

func (v *Validator) CheckStringLengthMin(token interface{}, s string, min int) bool {
	return v.Check(token, len(s) >= min, "string_too_short",
		"string length must be greater or equal to %d", min)
}

func (v *Validator) CheckStringLengthMax(token interface{}, s string, max int) bool {
	return v.Check(token, len(s) <= max, "string_too_long",
		"string length must be lower or equal to %d", max)
}

func (v *Validator) CheckStringLengthMinMax(token interface{}, s string, min, max int) bool {
	if !v.CheckStringLengthMin(token, s, min) {
		return false
	}

	return v.CheckStringLengthMax(token, s, max)
}

func (v *Validator) CheckStringNotEmpty(token interface{}, s string) bool {
	return v.Check(token, s != "", "missing_or_empty_string",
		"missing or empty string")
}

func (v *Validator) CheckStringValue(token interface{}, value interface{}, values interface{}) bool {
	valueType := reflect.TypeOf(value)
	if valueType.Kind() != reflect.String {
		panic(fmt.Sprintf("value %#v (%T) is not a string", value, value))
	}

	s := reflect.ValueOf(value).String()

	valuesType := reflect.TypeOf(values)
	if valuesType.Kind() != reflect.Slice {
		panic(fmt.Sprintf("values %#v (%T) are not a slice", values, values))
	}
	if valuesType.Elem().Kind() != reflect.String {
		panic(fmt.Sprintf("values %#v (%T) are not a slice of strings",
			values, values))
	}

	valuesValue := reflect.ValueOf(values)

	found := false
	for i := 0; i < valuesValue.Len(); i++ {
		s2 := valuesValue.Index(i).String()
		if s == s2 {
			found = true
		}
	}

	if !found {
		var buf bytes.Buffer

		buf.WriteString("value must be one of the following strings: ")

		for i := 0; i < valuesValue.Len(); i++ {
			if i > 0 {
				buf.WriteString(", ")
			}

			s2 := valuesValue.Index(i).String()
			buf.WriteString(s2)
		}

		v.AddError(token, "invalid_value", "%s", buf.String())
	}

	return found
}

func (v *Validator) CheckStringMatch(token interface{}, s string, re *regexp.Regexp) bool {
	return v.CheckStringMatch2(token, s, re, "invalid_string_format",
		"string must match the following regular expression: %s",
		re.String())
}

func (v *Validator) CheckStringMatch2(token interface{}, s string, re *regexp.Regexp, code, format string, args ...interface{}) bool {
	if !re.MatchString(s) {
		v.AddError(token, code, format, args...)
		return false
	}

	return true
}

func (v *Validator) CheckStringURI(token interface{}, s string) bool {
	// The url.Parse function parses URI references. Most of the time we are
	// interested in URIs, so we check that there is a schema.

	uri, err := url.Parse(s)
	if err != nil {
		v.AddError(token, "invalid_uri_format", "string must be a valid uri")
		return false
	}

	if uri.Scheme == "" {
		v.AddError(token, "missing_uri_scheme", "uri must have a scheme")
		return false
	}

	return true
}

func (v *Validator) CheckArrayLengthMin(token interface{}, value interface{}, min int) bool {
	var length int

	checkArray(value, &length)

	return v.Check(token, length >= min, "array_too_small",
		"array must contain %d or more elements", min)
}

func (v *Validator) CheckArrayLengthMax(token interface{}, value interface{}, max int) bool {
	var length int

	checkArray(value, &length)

	return v.Check(token, length <= max, "array_too_large",
		"array must contain %d or less elements", max)
}

func (v *Validator) CheckArrayLengthMinMax(token interface{}, value interface{}, min, max int) bool {
	if !v.CheckArrayLengthMin(token, value, min) {
		return false
	}

	return v.CheckArrayLengthMax(token, value, max)
}

func (v *Validator) CheckArrayNotEmpty(token interface{}, value interface{}) bool {
	var length int

	checkArray(value, &length)

	return v.Check(token, length > 0, "empty_array", "array must not be empty")
}

func checkArray(value interface{}, plen *int) {
	valueType := reflect.TypeOf(value)

	switch valueType.Kind() {
	case reflect.Slice:
		*plen = reflect.ValueOf(value).Len()

	case reflect.Array:
		*plen = valueType.Len()

	default:
		panic(fmt.Sprintf("value is not a slice or array"))
	}
}

func (v *Validator) CheckOptionalObject(token interface{}, value interface{}) bool {
	if !checkObject(value) {
		return true
	}

	return v.doCheckObject(token, value)
}

func (v *Validator) CheckObject(token interface{}, value interface{}) bool {
	if !checkObject(value) {
		v.AddError(token, "missing_value", "missing value")
		return false
	}

	return v.doCheckObject(token, value)
}

func (v *Validator) CheckObjectArray(token interface{}, value interface{}) bool {
	valueType := reflect.TypeOf(value)
	kind := valueType.Kind()

	if kind != reflect.Array && kind != reflect.Slice {
		panic(fmt.Sprintf("value %#v (%T) is not an array or slice",
			value, value))
	}

	ok := true

	v.WithChild(token, func() {
		values := reflect.ValueOf(value)

		for i := 0; i < values.Len(); i++ {
			child := values.Index(i).Interface()
			childOk := v.CheckObject(strconv.Itoa(i), child)
			ok = ok && childOk
		}
	})

	return ok
}

func (v *Validator) CheckObjectMap(token interface{}, value interface{}) bool {
	valueType := reflect.TypeOf(value)
	if valueType.Kind() != reflect.Map {
		panic(fmt.Sprintf("value %#v (%T) is not a map", value, value))
	}

	ok := true

	v.WithChild(token, func() {
		values := reflect.ValueOf(value)

		iter := values.MapRange()
		for iter.Next() {
			key := iter.Key()
			if key.Kind() != reflect.String {
				panic(fmt.Sprintf("value %#v (%T) is a map whose keys are "+
					"not strings", value, value))
			}
			keyString := key.Interface().(string)

			value := iter.Value().Interface()

			valueOk := v.CheckObject(keyString, value)
			ok = ok && valueOk
		}
	})

	return ok
}

func (v *Validator) doCheckObject(token interface{}, value interface{}) bool {
	nbErrors := len(v.Errors)

	value2, ok := value.(Validatable)
	if !ok {
		return true
	}

	v.Push(token)
	value2.ValidateJSON(v)
	v.Pop()

	return len(v.Errors) == nbErrors
}

func checkObject(value interface{}) bool {
	valueType := reflect.TypeOf(value)
	if valueType == nil {
		return false
	}

	if valueType.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("value %#v (%T) is not a pointer", value, value))
	}

	pointedValueType := valueType.Elem()
	if pointedValueType.Kind() != reflect.Struct {
		panic(fmt.Sprintf("value %#v (%T) is not a pointer to a structure",
			value, value))
	}

	return !reflect.ValueOf(value).IsZero()
}
