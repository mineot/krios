package scan

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
)

var sqlScannerType = reflect.TypeOf((*sql.Scanner)(nil)).Elem()

// implementsScanner reports whether values of t keep positional semantics in a
// mapped single-row Scan. Types such as time.Time or sql.NullString implement
// sql.Scanner on their pointer and are scanned positionally, never treated as
// entities.
func implementsScanner(t reflect.Type) bool {
	if t.Implements(sqlScannerType) {
		return true
	}

	return t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(sqlScannerType)
}

// entityDest classifies a single Scan destination. ok reports whether the
// destination is a struct pointer subject to entity mapping; a non-nil err
// means the destination was recognized as an entity but cannot be used.
//
// Structs implementing sql.Scanner and structs without exported fields keep
// positional semantics: they are common value types (sql.Null*, time.Time) and
// have no entity to derive.
func entityDest(dest any) (ent *entity.Entity, ok bool, err error) {
	if dest == nil {
		return nil, false, nil
	}

	v := reflect.ValueOf(dest)

	if v.Kind() != reflect.Pointer {
		return nil, false, nil
	}

	t := v.Type().Elem()

	if t.Kind() != reflect.Struct || implementsScanner(t) || !hasExportedFields(t) {
		return nil, false, nil
	}

	if v.IsNil() {
		return nil, true, fmt.Errorf("%w: destination is a nil pointer to %s", errs.ErrInvalidEntity, entity.TypeName(t))
	}

	ent, err = entity.OfType(t)
	return ent, true, err
}

// hasExportedFields reports whether t declares at least one exported field.
func hasExportedFields(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).PkgPath == "" {
			return true
		}
	}

	return false
}

// scanEntity fills the struct pointed to by dest from the current row. Because
// *sql.Row does not expose column names, the result set must match the mapped
// columns in quantity and declaration order. sql.ErrNoRows is preserved
// unchanged; other failures are contextualized with the table while wrapping
// the cause, and that contextualized error is recorded on the owning
// transaction so a blocked Commit reports the same error the consumer saw.
func (rw Row) scanEntity(ent *entity.Entity, dest any) error {
	elem := reflect.ValueOf(dest).Elem()
	indexes := ent.FieldIndexes()
	dests := make([]any, len(indexes))

	for i := range indexes {
		dests[i] = elem.Field(indexes[i]).Addr().Interface()
	}

	err := rw.r.Scan(dests...)

	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		rw.record(err)
		return err
	}

	mappedErr := fmt.Errorf("krios: map %s: %w", ent.Table(), errs.NormalizeTxError(err))
	rw.record(mappedErr)

	return mappedErr
}

// allDest validates the destination of Rows.All, returning the settable slice
// value and its struct element type.
func allDest(dest any) (reflect.Value, reflect.Type, error) {
	invalid := fmt.Errorf("%w: destination must be a non-nil pointer to a slice of structs", errs.ErrInvalidEntity)

	if dest == nil {
		return reflect.Value{}, nil, invalid
	}

	v := reflect.ValueOf(dest)

	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Slice {
		return reflect.Value{}, nil, invalid
	}

	elem := v.Elem().Type().Elem()

	if elem.Kind() != reflect.Struct {
		return reflect.Value{}, nil, invalid
	}

	return v.Elem(), elem, nil
}

// All consumes the remaining rows and stores them in the slice pointed to by
// dest, which must be *[]T with T a struct type. Result columns are matched to
// fields by name: exact match first, then case-insensitive. Columns without a
// mapped field are discarded and fields without a column stay zero. The
// iterator is always closed; on success the destination slice is replaced with
// the mapped rows, on failure it is left unchanged. Scan and iteration failures
// are recorded on the owning transaction and block Commit.
func (rw *Rows) All(dest any) error {
	if rw == nil || rw.r == nil {
		return errors.New("krios: rows are closed or invalid")
	}

	slice, elemType, err := allDest(dest)

	if err != nil {
		return err
	}

	ent, err := entity.OfType(elemType)

	if err != nil {
		return err
	}

	columns, err := rw.Columns()

	if err != nil {
		return err
	}

	fieldIndexes := ent.FieldIndexes()
	fields := make([]int, len(columns))

	for i, column := range columns {
		if idx, ok := ent.FieldIndex(column); ok {
			fields[i] = idx
		} else {
			fields[i] = -1
		}
	}

	mapped := reflect.MakeSlice(slice.Type(), 0, 0)

	for rw.Next() {
		elem := reflect.New(elemType)
		dests := make([]any, len(columns))

		for i := range columns {
			if fields[i] < 0 {
				dests[i] = new(any)
				continue
			}

			dests[i] = elem.Elem().Field(fieldIndexes[fields[i]]).Addr().Interface()
		}

		if err := rw.r.Scan(dests...); err != nil {
			mappedErr := fmt.Errorf("krios: map %s: %w", ent.Table(), err)
			rw.record(mappedErr)
			_ = rw.Close()

			return mappedErr
		}

		mapped = reflect.Append(mapped, elem.Elem())
	}

	if err := rw.Err(); err != nil {
		_ = rw.Close()
		return err
	}

	if err := rw.Close(); err != nil {
		return err
	}

	slice.Set(mapped)
	return nil
}
