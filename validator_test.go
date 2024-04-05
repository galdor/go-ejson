package ejson

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestFoo struct {
	String   string
	Bar      *TestBar
	Bars     []*TestBar
	BarTable map[string]*TestBar
	Tag      string
}

type TestBar struct {
	Integers []int
}

func (foo *TestFoo) ValidateJSON(v *Validator) {
	v.CheckStringLengthMin("String", foo.String, 3)
	v.CheckOptionalObject("Bar", foo.Bar)
	v.CheckObjectArray("Bars", foo.Bars)
	v.CheckObjectMap("BarTable", foo.BarTable)
	v.CheckStringValue("Tag", foo.Tag, []string{"", "a", "b", "c"})
}

func (bar *TestBar) ValidateJSON(v *Validator) {
	v.WithChild("Integers", func() {
		for i, integer := range bar.Integers {
			v.CheckIntMax(i, integer, 10)
		}
	})
}

func TestValidate(t *testing.T) {
	assert := assert.New(t)

	var data TestFoo
	var err error
	var validationErrs ValidationErrors
	var validationErr *ValidationError

	// Valid data
	data = TestFoo{
		String: "abcdef",
		Bar:    &TestBar{Integers: []int{1, 2, 3}},
		Bars: []*TestBar{
			{Integers: []int{4}},
			{Integers: []int{5, 6}},
		},
		BarTable: map[string]*TestBar{
			"foo": {Integers: []int{4}},
			"bar": {Integers: []int{5, 6}},
		},
	}

	assert.NoError(Validate(&data))

	// Valid data with null optional object
	data = TestFoo{
		String: "abcdef",
		Bar:    nil,
	}

	assert.NoError(Validate(&data))

	// Simple top-level violation
	data = TestFoo{
		String: "ab",
		Bar:    nil,
	}

	err = Validate(&data)

	if assert.ErrorAs(err, &validationErrs) {
		if assert.Equal(1, len(validationErrs)) {
			validationErr = validationErrs[0]
			assert.Equal("/String", validationErr.Pointer.String())
			assert.Equal("string_too_short", validationErr.Code)
		}
	}

	// String value violation
	data = TestFoo{
		String: "abcdef",
		Tag:    "xyz",
	}

	err = Validate(&data)

	if assert.ErrorAs(err, &validationErrs) {
		if assert.Equal(1, len(validationErrs)) {
			validationErr = validationErrs[0]
			assert.Equal("/Tag", validationErr.Pointer.String())
			assert.Equal("invalid_value", validationErr.Code)
		}
	}

	// Null objects in an object array
	data = TestFoo{
		String: "abcdef",
		Bars: []*TestBar{
			{Integers: []int{4}},
			nil,
			{Integers: []int{5, 6}},
			nil,
		},
	}

	err = Validate(&data)

	if assert.ErrorAs(err, &validationErrs) {
		if assert.Equal(2, len(validationErrs)) {
			validationErr = validationErrs[0]
			assert.Equal("/Bars/1", validationErr.Pointer.String())
			assert.Equal("missing_value", validationErr.Code)

			validationErr = validationErrs[1]
			assert.Equal("/Bars/3", validationErr.Pointer.String())
			assert.Equal("missing_value", validationErr.Code)
		}
	}

	// Nested violations
	data = TestFoo{
		String: "abcdef",
		Bars: []*TestBar{
			nil,
			{Integers: []int{15}},
			{Integers: []int{5, 20}},
		},
		BarTable: map[string]*TestBar{
			"foo": {Integers: []int{15}},
		},
	}

	err = Validate(&data)

	if assert.ErrorAs(err, &validationErrs) {
		if assert.Equal(4, len(validationErrs)) {
			validationErr = validationErrs[0]
			assert.Equal("/Bars/0", validationErr.Pointer.String())
			assert.Equal("missing_value", validationErr.Code)

			validationErr = validationErrs[1]
			assert.Equal("/Bars/1/Integers/0", validationErr.Pointer.String())
			assert.Equal("integer_too_large", validationErr.Code)

			validationErr = validationErrs[2]
			assert.Equal("/Bars/2/Integers/1", validationErr.Pointer.String())
			assert.Equal("integer_too_large", validationErr.Code)

			validationErr = validationErrs[3]
			assert.Equal("/BarTable/foo/Integers/0",
				validationErr.Pointer.String())
			assert.Equal("integer_too_large", validationErr.Code)
		}
	}

	// Invalid top-level type
	err = Unmarshal([]byte(`42`), &data)

	if assert.ErrorAs(err, &validationErrs) {
		if assert.Equal(1, len(validationErrs)) {
			validationErr = validationErrs[0]
			assert.Equal("", validationErr.Pointer.String())
			assert.Equal("invalid_value_type", validationErr.Code)
		}
	}

	// Invalid member type
	err = Unmarshal([]byte(`{"String": 42}`), &data)

	if assert.ErrorAs(err, &validationErrs) {
		if assert.Equal(1, len(validationErrs)) {
			validationErr = validationErrs[0]
			assert.Equal("/String", validationErr.Pointer.String())
			assert.Equal("invalid_value_type", validationErr.Code)
		}
	}

	// Invalid nested member type
	//
	// The standard JSON parser returns the error on the array, nothing we can
	// do about it.
	err = Unmarshal([]byte(`{"String": "abcd", "Bars": [{"Integers": true}]}`),
		&data)

	if assert.ErrorAs(err, &validationErrs) {
		if assert.Equal(1, len(validationErrs)) {
			validationErr = validationErrs[0]
			assert.Equal("/Bars/Integers", validationErr.Pointer.String())
			assert.Equal("invalid_value_type", validationErr.Code)
		}
	}
}

func TestValidateDNSLabel(t *testing.T) {
	tests := []struct {
		s     string
		valid bool
		code  string
	}{
		{"a", true, ""},
		{"abc", true, ""},
		{"abc-def", true, ""},
		{"012-345", true, ""},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			true, ""}, // 63 characters

		{"", false, "missing_or_empty_string"},
		{"-", false, "invalid_dns_label"},
		{"-abc", false, "invalid_dns_label"},
		{"abc-", false, "invalid_dns_label"},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			false, "dns_label_too_long"}, // 64 characters
	}

	for _, test := range tests {
		v := NewValidator()
		valid := v.CheckDNSLabel("test", test.s)

		var code string
		if !valid {
			if len(v.Errors) == 0 {
				t.Errorf("validation failed without any validation error")
				continue
			}

			code = v.Errors[0].Code
		}

		switch {
		case valid && !test.valid:
			t.Errorf("validation of string %q succeeded but should have "+
				"failed with code %q", test.s, test.code)

		case !valid && test.valid:
			t.Errorf("validation of string %q failed with code %q",
				test.s, code)

		case !valid && test.code != code:
			t.Errorf("validation of string %q failed with code %q but "+
				"should have failed with code %q", test.s, code, test.code)
		}
	}
}
