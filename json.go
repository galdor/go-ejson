package ejson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// It would be nice to have a DecodeStrict() function which would use
// (*json.Decoder).DisallowUnknownFields(). Infortunately, the errors produced
// this way are not structured and therefore unusable. There is no way out
// this whole mess without rewriting an unmarshaller based on
// (*json.Decoder).Token(). This would increase memory pressure, but this is
// irrelevant for most use cases and would allow much better error reporting.

func Unmarshal(data []byte, dest interface{}) error {
	d := json.NewDecoder(bytes.NewReader(data))
	return UnmarshalDecoder(d, dest)
}

func UnmarshalDecoder(d *json.Decoder, dest interface{}) error {
	if err := d.Decode(dest); err != nil {
		return ConvertUnmarshallingError(err)
	}

	return Validate(dest)

}

func UnmarshalReader(r io.Reader, dest interface{}) error {
	d := json.NewDecoder(r)
	return UnmarshalDecoder(d, dest)
}

func ConvertUnmarshallingError(err error) error {
	switch err2 := err.(type) {
	case *json.UnmarshalTypeError:
		var pointer Pointer

		if err2.Field != "" {
			parts := strings.Split(err2.Field, ".")

			// The Go type system is still completely broken
			parts2 := make([]interface{}, len(parts))
			for i, p := range parts {
				parts2[i] = p
			}

			pointer = NewPointer(parts2...)
		}

		message := fmt.Sprintf("cannot decode %v into value of type %v",
			err2.Value, err2.Type)

		return ValidationErrors{
			&ValidationError{
				Pointer: pointer,
				Code:    "invalid_value_type",
				Message: message,
			},
		}

	default:
		return err
	}
}
