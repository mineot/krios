package entity

import (
	"database/sql/driver"
	"fmt"
	"reflect"

	"github.com/mineot/krios/internal/errs"
)

// WriteValue is the disposition of one mapped column for a write operation
// (E06-P01 §4). INSERT writes every column: Absent binds NULL. UPDATE skips
// Absent columns and writes NULL for a nil Value produced by a Valuer.
type WriteValue struct {
	// Column is the mapped column name.
	Column string
	// Value is the value to bind. It is nil only for an explicit NULL.
	Value any
	// Absent marks a nil pointer field: INSERT binds NULL, UPDATE does not
	// write the column at all.
	Absent bool
}

// WriteValues returns the write disposition of every mapped column of record
// in field declaration order. record must be T or *T of the entity's type; any
// other value (including nil) fails with errs.ErrInvalidEntity.
//
// Values follow E06-P01 §4.2: a non-pointer field yields its value, including
// zero values; a non-nil pointer yields the pointed-to value; a nil pointer is
// reported as Absent; a field implementing driver.Valuer (such as sql.Null*
// with Valid false) yields NULL as its Value. A rejection by a Valuer fails
// with errs.ErrInvalidEntity, preserving the cause.
func (e *Entity) WriteValues(record any) ([]WriteValue, error) {
	v, err := e.recordValue(record)

	if err != nil {
		return nil, err
	}

	values := make([]WriteValue, len(e.fields))

	for i := range e.fields {
		f := &e.fields[i]
		fv := v.Field(f.index)
		wv := WriteValue{Column: f.name}

		for fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				wv.Absent = true
				values[i] = wv
				break
			}
			fv = fv.Elem()
		}

		if wv.Absent {
			continue
		}

		if valuer, ok := valuerOf(fv); ok {
			produced, err := valuer.Value()

			if err != nil {
				return nil, fmt.Errorf("%w: %s: column %q: %w", errs.ErrInvalidEntity, e.table, f.name, err)
			}

			wv.Value = produced
			values[i] = wv
			continue
		}

		wv.Value = fv.Interface()
		values[i] = wv
	}

	return values, nil
}

// valuerOf obtains the driver.Valuer implemented by fv, preferring the address
// so pointer-receiver methods can be used on addressable fields.
func valuerOf(fv reflect.Value) (driver.Valuer, bool) {
	if fv.CanAddr() {
		if valuer, ok := fv.Addr().Interface().(driver.Valuer); ok {
			return valuer, true
		}
	}

	valuer, ok := fv.Interface().(driver.Valuer)
	return valuer, ok
}
