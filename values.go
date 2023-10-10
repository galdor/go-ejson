package ejson

import "fmt"

type InvalidValueError struct {
	Value interface{}
}

func (err *InvalidValueError) Error() string {
	return fmt.Sprintf("%#v (%T) is not a valid json value",
		err.Value, err.Value)
}

func IsNumber(v interface{}) bool {
	_, ok := v.(float64)
	return ok
}

func IsString(v interface{}) bool {
	_, ok := v.(string)
	return ok
}

func IsBoolean(v interface{}) bool {
	_, ok := v.(bool)
	return ok
}

func IsArray(v interface{}) bool {
	_, ok := v.([]interface{})
	return ok
}

func IsObject(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}

func AsNumber(v interface{}) float64 {
	return v.(float64)
}

func AsString(v interface{}) string {
	return v.(string)
}

func AsBoolean(v interface{}) bool {
	return v.(bool)
}

func AsArray(v interface{}) []interface{} {
	return v.([]interface{})
}

func AsObject(v interface{}) map[string]interface{} {
	return v.(map[string]interface{})
}

func Equal(v1, v2 interface{}) bool {
	switch {
	case IsNumber(v1) && IsNumber(v2):
		return AsNumber(v1) == AsNumber(v2)

	case IsString(v1) && IsString(v2):
		return AsString(v1) == AsString(v2)

	case IsBoolean(v1) && IsBoolean(v2):
		return AsBoolean(v1) == AsBoolean(v2)

	case IsArray(v1) && IsArray(v2):
		a1 := AsArray(v1)
		a2 := AsArray(v2)

		if len(a1) != len(a2) {
			return false
		}

		for i := 0; i < len(a1); i++ {
			if !Equal(a1[i], a2[i]) {
				return false
			}
		}

		return true

	case IsObject(v1) && IsObject(v2):
		obj1 := AsObject(v1)
		obj2 := AsObject(v2)

		for key, value1 := range obj1 {
			value2, found := obj2[key]
			if !found || !Equal(value1, value2) {
				return false
			}
		}

		for key, value2 := range obj2 {
			value1, found := obj1[key]
			if !found || !Equal(value1, value2) {
				return false
			}
		}

		return true
	}

	return false
}

func ObjectKeys(v interface{}) []string {
	obj := AsObject(v)

	keys := make([]string, len(obj))

	i := 0
	for key := range obj {
		keys[i] = key
		i++
	}

	return keys
}

func ObjectValues(v interface{}) []interface{} {
	obj := AsObject(v)

	values := make([]interface{}, len(obj))

	i := 0
	for _, value := range obj {
		values[i] = value
		i++
	}

	return values
}
