package entity

import (
	"fmt"
	"reflect"

	"github.com/mineot/krios/internal/errs"
)

// Validator is implemented by field types that validate their own value.
// Validate returns nil when the value is valid. Krios calls it through
// Entity.Validate and Entity.ValidateAll; E06 writers reuse the same engine.
type Validator interface {
	Validate() error
}

// Formatter is implemented by field types that provide a presentation text.
// The returned text is never used in database operations; those always use the
// original field value.
type Formatter interface {
	Format() (string, error)
}

var (
	validatorType = reflect.TypeOf((*Validator)(nil)).Elem()
	formatterType = reflect.TypeOf((*Formatter)(nil)).Elem()
)

// implementsInterface reports whether values of t implement iface, considering
// both the value type and its pointer (method sets of value and pointer
// receivers).
func implementsInterface(t, iface reflect.Type) bool {
	if t.Implements(iface) {
		return true
	}

	return t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(iface)
}

// isNilValue reports whether v is a nil pointer, interface, map, slice, func or
// channel. Nil fields are skipped by validation and format the empty text.
func isNilValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return v.IsNil()
	default:
		return false
	}
}

// recordValue normalizes record (T or *T of the entity's type) to an addressable
// struct value so pointer-receiver methods can be called.
func (e *Entity) recordValue(record any) (reflect.Value, error) {
	if record == nil {
		return reflect.Value{}, e.invalidRecord("value or pointer")
	}

	v := reflect.ValueOf(record)

	if v.Kind() == reflect.Pointer {
		if v.IsNil() || v.Type().Elem() != e.typ {
			return reflect.Value{}, e.invalidRecord("value or pointer")
		}

		return v.Elem(), nil
	}

	if v.Type() != e.typ {
		return reflect.Value{}, e.invalidRecord("value or pointer")
	}

	copy := reflect.New(e.typ)
	copy.Elem().Set(v)

	return copy.Elem(), nil
}

// sliceValue normalizes records ([]T or *[]T of the entity's type) to a slice
// value.
func (e *Entity) sliceValue(records any) (reflect.Value, error) {
	if records == nil {
		return reflect.Value{}, e.invalidRecord("slice or pointer to slice")
	}

	v := reflect.ValueOf(records)

	if v.Kind() == reflect.Pointer {
		if v.IsNil() || v.Elem().Kind() != reflect.Slice {
			return reflect.Value{}, e.invalidRecord("slice or pointer to slice")
		}

		v = v.Elem()
	}

	if v.Kind() != reflect.Slice || v.Type().Elem() != e.typ {
		return reflect.Value{}, e.invalidRecord("slice or pointer to slice")
	}

	return v, nil
}

func (e *Entity) invalidRecord(want string) error {
	return fmt.Errorf("%w: %s requires a %s of type %s", errs.ErrInvalidEntity, e.table, want, TypeName(e.typ))
}

// Validate checks every mapped field of record that implements Validator,
// in declaration order. record must be T or *T of the entity's type. It returns
// the first failure wrapped with errs.ErrValidation, the table and the column; nil
// fields are skipped.
func (e *Entity) Validate(record any) error {
	v, err := e.recordValue(record)

	if err != nil {
		return err
	}

	return e.validateValue(v)
}

// ValidateAll validates each element of records ([]T or *[]T) in order,
// stopping at the first invalid record. It returns the first failure wrapped
// with errs.ErrValidation, the table and the column.
func (e *Entity) ValidateAll(records any) error {
	slice, err := e.sliceValue(records)

	if err != nil {
		return err
	}

	for i := 0; i < slice.Len(); i++ {
		if err := e.validateValue(slice.Index(i)); err != nil {
			return err
		}
	}

	return nil
}

func (e *Entity) validateValue(v reflect.Value) error {
	for i := range e.fields {
		f := &e.fields[i]

		if !f.validator {
			continue
		}

		fv := v.Field(f.index)

		if isNilValue(fv) {
			continue
		}

		validator, ok := validatorOf(fv)

		if !ok {
			continue
		}

		if err := validator.Validate(); err != nil {
			return fmt.Errorf("%w: %s: column %q: %w", errs.ErrValidation, e.table, f.name, err)
		}
	}

	return nil
}

// Format returns the presentation text of every mapped field of record, keyed
// by column. record must be T or *T of the entity's type. Fields implementing
// Formatter use its text; other fields fall back to fmt.Sprint. Nil pointer
// fields format the empty text. The original values are never changed.
func (e *Entity) Format(record any) (map[string]string, error) {
	v, err := e.recordValue(record)

	if err != nil {
		return nil, err
	}

	formatted := make(map[string]string, len(e.fields))

	for i := range e.fields {
		f := &e.fields[i]
		fv := v.Field(f.index)

		if f.formatter && !isNilValue(fv) {
			if formatter, ok := formatterOf(fv); ok {
				text, err := formatter.Format()

				if err != nil {
					return nil, fmt.Errorf("%w: %s: column %q: %w", errs.ErrFormat, e.table, f.name, err)
				}

				formatted[f.name] = text
				continue
			}
		}

		if isNilValue(fv) {
			formatted[f.name] = ""
			continue
		}

		text, ok := stringValue(fv)

		if !ok {
			text = ""
		}

		formatted[f.name] = text
	}

	return formatted, nil
}

// stringValue unwraps pointer and interface values so the presentation text
// reflects the pointed-to value rather than its address. A nil pointer is
// reported as absent.
func stringValue(v reflect.Value) (string, bool) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return "", false
		}
		v = v.Elem()
	}

	return fmt.Sprint(v.Interface()), true
}

// validatorOf obtains the Validator implemented by fv, preferring the address
// so pointer-receiver methods can be used on addressable fields.
func validatorOf(fv reflect.Value) (Validator, bool) {
	if fv.CanAddr() {
		if validator, ok := fv.Addr().Interface().(Validator); ok {
			return validator, true
		}
	}

	validator, ok := fv.Interface().(Validator)

	return validator, ok
}

// formatterOf obtains the Formatter implemented by fv, preferring the address
// so pointer-receiver methods can be used on addressable fields.
func formatterOf(fv reflect.Value) (Formatter, bool) {
	if fv.CanAddr() {
		if formatter, ok := fv.Addr().Interface().(Formatter); ok {
			return formatter, true
		}
	}

	formatter, ok := fv.Interface().(Formatter)

	return formatter, ok
}
