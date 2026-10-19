package core

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode"
)

// Entity is the descriptor derived by reflection from a Go struct type. It
// holds the table name, the mapped columns in declaration order and the
// primary key markers. Instances are produced by EntityOf, cached per type and
// safe for concurrent use; treat exported results as read-only.
type Entity struct {
	table        string
	typ          reflect.Type
	fields       []entityField
	byColumn     map[string]int
	byColumnFold map[string]int
	primaryKeys  []string
}

// entityField binds a mapped column to the struct field that receives it.
type entityField struct {
	name  string
	index int
	pk    bool
}

// entityResult is the cached derivation outcome: either an entity or the
// error produced while deriving it, so a type is inspected only once.
type entityResult struct {
	entity *Entity
	err    error
}

var entityCache sync.Map // reflect.Type -> *entityResult

// EntityOf derives the entity descriptor for T, which may be a struct type or
// a pointer to a struct type. The result is cached per type. Invalid structs
// (unsupported fields, duplicate columns, bad tags or TableName) return an
// error matching ErrInvalidEntity.
func EntityOf[T any]() (*Entity, error) {
	return entityOfType(reflect.TypeOf((*T)(nil)).Elem())
}

func entityOfType(t reflect.Type) (*Entity, error) {
	t = indirectType(t)

	if t == nil || t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: %s is not a struct type", ErrInvalidEntity, typeName(t))
	}

	if cached, ok := entityCache.Load(t); ok {
		res := cached.(*entityResult)
		return res.entity, res.err
	}

	// Derivation is pure; concurrent callers may derive twice and the first
	// stored result wins.
	entity, err := deriveEntity(t)
	cached, _ := entityCache.LoadOrStore(t, &entityResult{entity: entity, err: err})
	res := cached.(*entityResult)
	return res.entity, res.err
}

func indirectType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}

func typeName(t reflect.Type) string {
	if t == nil {
		return "<nil>"
	}

	return t.String()
}

func deriveEntity(t reflect.Type) (*Entity, error) {
	table, err := deriveTableName(t)

	if err != nil {
		return nil, err
	}

	e := &Entity{
		table:        table,
		typ:          t,
		byColumn:     make(map[string]int),
		byColumnFold: make(map[string]int),
	}

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		if sf.Anonymous {
			return nil, invalidEntity(t, "embedded field %q is not supported", sf.Name)
		}

		tag, hasTag := sf.Tag.Lookup("krios")

		if sf.PkgPath != "" {
			if hasTag {
				return nil, invalidEntity(t, "unexported field %q must not carry a krios tag", sf.Name)
			}

			continue
		}

		name, pk, skip, tagErr := parseFieldTag(sf.Name, tag, hasTag)
		if tagErr != nil {
			return nil, invalidEntity(t, "field %q: %v", sf.Name, tagErr)
		}

		if skip {
			continue
		}

		if _, dup := e.byColumn[name]; dup {
			return nil, invalidEntity(t, "duplicate column %q", name)
		}

		fold := strings.ToLower(name)

		if _, dup := e.byColumnFold[fold]; dup {
			return nil, invalidEntity(t, "duplicate column %q ignoring case", name)
		}

		idx := len(e.fields)
		e.fields = append(e.fields, entityField{name: name, index: i, pk: pk})
		e.byColumn[name] = idx
		e.byColumnFold[fold] = idx

		if pk {
			e.primaryKeys = append(e.primaryKeys, name)
		}
	}

	if len(e.fields) == 0 {
		return nil, invalidEntity(t, "no mapped fields")
	}

	return e, nil
}

// parseFieldTag interprets the krios tag of a field. Fields without a tag (or
// with an empty one) derive their column name from the field name.
func parseFieldTag(fieldName, tag string, hasTag bool) (name string, pk, skip bool, err error) {
	if !hasTag || tag == "" {
		return snakeCase(fieldName), false, false, nil
	}

	parts := strings.Split(tag, ",")
	column := strings.TrimSpace(parts[0])

	if column == "-" {
		return "", false, true, nil
	}

	for _, option := range parts[1:] {
		switch strings.TrimSpace(option) {
		case "":
		case "pk":
			pk = true
		default:
			return "", false, false, fmt.Errorf("unsupported option %q in krios tag %q", option, tag)
		}
	}

	if column == "" {
		column = snakeCase(fieldName)
	}

	return column, pk, false, nil
}

// deriveTableName returns the table name for t: the TableName() string method
// when present, otherwise the snake_case name of the type.
func deriveTableName(t reflect.Type) (string, error) {
	if method := reflect.New(t).MethodByName("TableName"); method.IsValid() {
		mt := method.Type()

		if mt.NumIn() != 0 || mt.NumOut() != 1 || mt.Out(0).Kind() != reflect.String {
			return "", invalidEntity(t, "TableName must have signature func() string")
		}

		name := strings.TrimSpace(method.Call(nil)[0].String())

		if name == "" {
			return "", invalidEntity(t, "TableName returned an empty name")
		}

		return name, nil
	}

	if t.Name() == "" {
		return "", invalidEntity(t, "anonymous struct requires a TableName method")
	}

	return snakeCase(t.Name()), nil
}

func invalidEntity(t reflect.Type, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrInvalidEntity, typeName(t), fmt.Sprintf(format, args...))
}

// snakeCase converts an exported field or type name to snake_case, splitting
// acronym runs from following words: UserID -> user_id, HTTPServer ->
// http_server.
func snakeCase(name string) string {
	runes := []rune(name)
	var b strings.Builder
	b.Grow(len(runes) + 4)

	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextIsLower) {
				b.WriteByte('_')
			}
		}

		b.WriteRune(unicode.ToLower(r))
	}

	return b.String()
}

// Table returns the mapped table name.
func (e *Entity) Table() string { return e.table }

// Type returns the struct type the entity was derived from.
func (e *Entity) Type() reflect.Type { return e.typ }

// Columns returns the mapped column names in field declaration order. The
// returned slice is a copy.
func (e *Entity) Columns() []string {
	columns := make([]string, len(e.fields))
	for i := range e.fields {
		columns[i] = e.fields[i].name
	}
	return columns
}

// PrimaryKeys returns the columns marked with pk, in declaration order. The
// returned slice is a copy and is empty when the entity declares no key.
func (e *Entity) PrimaryKeys() []string {
	return append([]string(nil), e.primaryKeys...)
}

// fieldIndex resolves a result set column name to the mapped field position,
// trying an exact match first and then a case-insensitive one.
func (e *Entity) fieldIndex(column string) (int, bool) {
	if idx, ok := e.byColumn[column]; ok {
		return idx, true
	}
	if idx, ok := e.byColumnFold[strings.ToLower(column)]; ok {
		return idx, true
	}
	return 0, false
}
