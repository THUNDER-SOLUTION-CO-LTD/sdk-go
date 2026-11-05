package thunder

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

// Field preserves the difference between an absent field, null, and a typed value.
// Its zero value means absent. Use Some or Null to construct an explicit value.
type Field[T any] struct {
	value   T
	present bool
	null    bool
}

// Some constructs a present non-null field.
func Some[T any](value T) Field[T] { return Field[T]{value: value, present: true} }

// Null constructs an explicitly null field.
func Null[T any]() Field[T] { return Field[T]{present: true, null: true} }

// IsSet reports whether the API returned the field or the caller explicitly set it.
func (f Field[T]) IsSet() bool { return f.present }

// IsNull reports an explicitly null field, not an absent field.
func (f Field[T]) IsNull() bool { return f.present && f.null }

// Value returns the typed value and whether the field is present and non-null.
func (f Field[T]) Value() (T, bool) { return f.value, f.present && !f.null }

// UnmarshalJSON decodes a present field while preserving explicit null.
func (f *Field[T]) UnmarshalJSON(raw []byte) error {
	var value T
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		*f = Null[T]()
		return nil
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	*f = Some(value)
	return nil
}

// MarshalJSON encodes the field value. Enclosing SDK models omit absent fields.
func (f Field[T]) MarshalJSON() ([]byte, error) {
	if !f.present || f.null {
		return []byte("null"), nil
	}
	return json.Marshal(f.value)
}

func marshalFields(model any) ([]byte, error) {
	value := reflect.ValueOf(model)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return []byte("null"), nil
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return nil, errors.New("model must be a struct")
	}
	data := make(map[string]any)
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag := strings.Split(field.Tag.Get("json"), ",")
		if tag[0] == "-" {
			continue
		}
		name := tag[0]
		if name == "" {
			name = field.Name
		}
		current := value.Field(i).Interface()
		if optional, ok := current.(interface{ IsSet() bool }); ok && !optional.IsSet() {
			continue
		}
		data[name] = current
	}
	return json.Marshal(data)
}
