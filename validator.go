package ejson

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.n16f.net/uuid"
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

	buf.WriteString("invalid data")

	if len(errs) > 0 {
		buf.WriteByte(':')
	}

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

func (v *Validator) CheckInt64Min(token interface{}, i, min int64) bool {
	return v.Check(token, i >= min, "integer_too_small",
		"integer must be greater or equal to %d", min)
}

func (v *Validator) CheckInt64Max(token interface{}, i, max int64) bool {
	return v.Check(token, i <= max, "integer_too_large",
		"integer must be lower or equal to %d", max)
}

func (v *Validator) CheckInt64MinMax(token interface{}, i, min, max int64) bool {
	if !v.CheckInt64Min(token, i, min) {
		return false
	}

	return v.CheckInt64Max(token, i, max)
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
	length := utf8.RuneCountInString(s)
	return v.Check(token, length >= min, "string_too_short",
		"string length must be greater or equal to %d", min)
}

func (v *Validator) CheckStringLengthMax(token interface{}, s string, max int) bool {
	length := utf8.RuneCountInString(s)
	return v.Check(token, length <= max, "string_too_long",
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

func (v *Validator) CheckUUID(token interface{}, value interface{}) bool {
	var id uuid.UUID

	switch value2 := value.(type) {
	case string:
		if !v.CheckStringNotEmpty(token, value2) {
			return false
		}

		ok := v.Check(token, id.Parse(value2) == nil, "invalid_uuid",
			"string must be a valid uuid")
		if !ok {
			return false
		}

	case uuid.UUID:
		id = value2
	}

	return v.Check(token, !id.Equal(uuid.Nil), "missing_or_null_uuid",
		"missing or null uuid")
}

func (v *Validator) CheckNetworkAddress(token any, s string) {
	_, portString, err := net.SplitHostPort(s)
	if err != nil {
		var msg string
		var addrErr *net.AddrError

		if errors.As(err, &addrErr) {
			msg = addrErr.Err
		} else {
			msg = err.Error()
		}

		v.AddError(token, "invalid_address", "invalid address: %v", msg)
		return
	}

	if portString == "" {
		v.AddError(token, "empty_port_number", "empty port number")
	} else {
		port, err := strconv.ParseInt(portString, 10, 64)
		if err != nil {
			v.AddError(token, "invalid_port_number", "invalid port number")
		} else if port < 1 {
			v.AddError(token, "invalid_port_number",
				"port number must be greater than 0")
		} else if port >= 65535 {
			v.AddError(token, "invalid_port_number",
				"port number must be lower than 65535")
		}
	}
}

func (v *Validator) CheckDomainName(token any, s string) {
	addError := func(format string, args ...any) {
		v.AddError(token, "invalid_domain_name", format, args...)
	}

	// If it is an IP address, it is a valid domain but not a valid domain name
	if net.ParseIP(s) != nil {
		addError("IP address is not a valid domain name")
		return
	}

	// RFC 952 DOD INTERNET HOST TABLE SPECIFICATION
	//
	// <domainname> ::= <hname>
	// <hname> ::= <name>*["."<name>]
	// <name>  ::= <let>[*[<let-or-digit-or-hyphen>]<let-or-digit>]

	// RFC 1034 3.5. Preferred name syntax
	//
	// "Labels must be 63 characters or less"

	// RFC 1123 2. GENERAL ISSUES
	//
	// "the restriction on the first character is relaxed to allow either a
	// letter or a digit. Host software MUST support this more liberal syntax."

	const maxLabelLength = 63

	isLetter := func(c byte) bool {
		return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
	}

	isDigit := func(c byte) bool {
		return c >= '0' && c <= '9'
	}

	labels := strings.Split(s, ".")
labelLoop:
	for _, label := range labels {
		if len(label) == 0 {
			addError("invalid empty domain name label")
			return
		}

		for i := range len(label) {
			if label[i] > 0x7f {
				addError("domain name labels must only contain 7-bit ASCII " +
					"characters")
				continue labelLoop
			}
		}

		if len(label) > maxLabelLength {
			addError("domain name label must be %d character long at most",
				maxLabelLength)
			return
		}

		if c := label[0]; !(isLetter(c) || isDigit(c)) {
			addError("domain name label must start with a letter or digit")
		}

		if c := label[len(label)-1]; !(isLetter(c) || isDigit(c)) {
			addError("domain name label must end with a letter or digit")
		}

		for i := 1; i < len(label)-1; i++ {
			if c := label[i]; !(isLetter(c) || isDigit(c) || c == '-') {
				addError("domain name label character must be a letter, " +
					"a digit or a '-' character")
			}
		}
	}
}

func (v *Validator) CheckEmailAddress(token any, s string) {
	// Email validation is one of the most nitpicked subject in the software
	// industry. We keep validation to a minimum: one can always write a more
	// stringent method if needs be.

	addError := func(format string, args ...any) {
		v.AddError(token, "invalid_email_address", format, args...)
	}

	localPart, domain, found := strings.Cut(s, "@")
	if !found {
		addError("missing '@' separator")
		return
	}

	if len(domain) == 0 {
		addError("invalid empty domain")
	}

	if len(localPart) == 0 {
		addError("invalid empty local part")
	}
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
		v.AddError(token, "missing_or_null_value", "missing or null value")
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
