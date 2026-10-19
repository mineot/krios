package core

import (
	"errors"
	"sync/atomic"
	"testing"
)

type metadataUser struct {
	ID   int64 `krios:"id,pk"`
	Name string
	Bio  *string `krios:"bio"`
	Skip string  `krios:"-"`
}

type defaultKey struct {
	Account string `krios:",pk"`
	Value   int
}

type acronymUser struct {
	UserID     int64
	HTTPServer string
	ID         int64
}

type compositeKey struct {
	TenantID int64 `krios:"tenant_id,pk"`
	ID       int64 `krios:"id,pk"`
	Name     string
}

type namedTable struct{ Value int }

func (namedTable) TableName() string { return "custom_table" }

type pointerTable struct{ Value int }

func (*pointerTable) TableName() string { return "pointer_table" }

type badTableSignature struct{ Value int }

func (badTableSignature) TableName() int { return 1 }

type emptyTableName struct{ Value int }

func (emptyTableName) TableName() string { return "" }

type countedTable struct{ Value int }

var countedTableCalls atomic.Int32

func (countedTable) TableName() string {
	countedTableCalls.Add(1)
	return "counted"
}

type invalidNoFields struct{ hidden int }

type invalidDuplicate struct {
	Left  int `krios:"side"`
	Right int `krios:"side"`
}

type invalidCaseDuplicate struct {
	Left  int `krios:"side"`
	Right int `krios:"SIDE"`
}

type invalidOption struct {
	Value int `krios:"value,sortable"`
}

type invalidTaggedUnexported struct {
	hidden int `krios:"hidden"`
	Public int
}

type invalidEmbeddedBase struct{ ID int }

type invalidEmbedded struct{ invalidEmbeddedBase }

func TestEntityOfDerivesMetadata(t *testing.T) {
	ent, err := EntityOf[metadataUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	if ent.Table() != "metadata_user" {
		t.Errorf("Table() = %q, want metadata_user", ent.Table())
	}
	want := []string{"id", "name", "bio"}
	if got := ent.Columns(); !equalStrings(got, want) {
		t.Errorf("Columns() = %v, want %v", got, want)
	}
	if got := ent.PrimaryKeys(); !equalStrings(got, []string{"id"}) {
		t.Errorf("PrimaryKeys() = %v, want [id]", got)
	}
	if ent.Type().Name() != "metadataUser" {
		t.Errorf("Type() = %v, want metadataUser", ent.Type())
	}
}

func TestEntityOfDefaultsAndTags(t *testing.T) {
	ent, err := EntityOf[defaultKey]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	if ent.Table() != "default_key" {
		t.Errorf("Table() = %q, want default_key", ent.Table())
	}
	if got := ent.Columns(); !equalStrings(got, []string{"account", "value"}) {
		t.Errorf("Columns() = %v, want [account value]", got)
	}
	if got := ent.PrimaryKeys(); !equalStrings(got, []string{"account"}) {
		t.Errorf("PrimaryKeys() = %v, want [account]", got)
	}

	composite, err := EntityOf[compositeKey]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	if got := composite.PrimaryKeys(); !equalStrings(got, []string{"tenant_id", "id"}) {
		t.Errorf("PrimaryKeys() = %v, want [tenant_id id]", got)
	}
}

func TestSnakeCase(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"User", "user"},
		{"UserProfile", "user_profile"},
		{"UserID", "user_id"},
		{"HTTPServer", "http_server"},
		{"OAuth2Token", "o_auth2_token"},
		{"ID", "id"},
		{"T", "t"},
		{"A1B2", "a1_b2"},
		{"already_snake", "already_snake"},
	} {
		if got := snakeCase(tc.in); got != tc.want {
			t.Errorf("snakeCase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	ent, err := EntityOf[acronymUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	want := []string{"user_id", "http_server", "id"}
	if got := ent.Columns(); !equalStrings(got, want) {
		t.Errorf("Columns() = %v, want %v", got, want)
	}
}

func TestEntityOfTableNameOverrides(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func() (*Entity, error)
		want string
	}{
		{"value receiver", func() (*Entity, error) { return EntityOf[namedTable]() }, "custom_table"},
		{"pointer receiver", func() (*Entity, error) { return EntityOf[*pointerTable]() }, "pointer_table"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ent, err := tc.call()
			if err != nil {
				t.Fatalf("EntityOf() = %v", err)
			}
			if ent.Table() != tc.want {
				t.Errorf("Table() = %q, want %q", ent.Table(), tc.want)
			}
		})
	}
}

func TestEntityOfInvalidStructs(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func() (*Entity, error)
	}{
		{"no fields", func() (*Entity, error) { return EntityOf[invalidNoFields]() }},
		{"duplicate column", func() (*Entity, error) { return EntityOf[invalidDuplicate]() }},
		{"duplicate ignoring case", func() (*Entity, error) { return EntityOf[invalidCaseDuplicate]() }},
		{"unknown tag option", func() (*Entity, error) { return EntityOf[invalidOption]() }},
		{"tagged unexported field", func() (*Entity, error) { return EntityOf[invalidTaggedUnexported]() }},
		{"embedded field", func() (*Entity, error) { return EntityOf[invalidEmbedded]() }},
		{"bad TableName signature", func() (*Entity, error) { return EntityOf[badTableSignature]() }},
		{"empty TableName", func() (*Entity, error) { return EntityOf[emptyTableName]() }},
		{"anonymous struct", func() (*Entity, error) { return EntityOf[struct{ Value int }]() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ent, err := tc.call()
			if !errors.Is(err, ErrInvalidEntity) {
				t.Fatalf("EntityOf() = %v, want ErrInvalidEntity", err)
			}
			if ent != nil {
				t.Errorf("EntityOf() = %v, want nil descriptor", ent)
			}
		})
	}
}

func TestEntityOfRejectsNonStructs(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func() (*Entity, error)
	}{
		{"int", func() (*Entity, error) { return EntityOf[int]() }},
		{"pointer to int", func() (*Entity, error) { return EntityOf[*int]() }},
		{"slice", func() (*Entity, error) { return EntityOf[[]byte]() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.call(); !errors.Is(err, ErrInvalidEntity) {
				t.Fatalf("EntityOf() = %v, want ErrInvalidEntity", err)
			}
		})
	}
}

func TestEntityOfIsCachedPerType(t *testing.T) {
	first, err := EntityOf[metadataUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	second, err := EntityOf[*metadataUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	if first != second {
		t.Fatal("pointer and value forms produced different descriptors")
	}

	countedTableCalls.Store(0)
	if _, err := EntityOf[countedTable](); err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	if _, err := EntityOf[countedTable](); err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	if calls := countedTableCalls.Load(); calls != 1 {
		t.Fatalf("TableName called %d times, want 1", calls)
	}
}

func TestEntityAccessorsReturnCopies(t *testing.T) {
	ent, err := EntityOf[metadataUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	columns := ent.Columns()
	columns[0] = "mutated"
	if got := ent.Columns()[0]; got != "id" {
		t.Errorf("Columns() after mutation = %q, want id", got)
	}

	keys := ent.PrimaryKeys()
	keys[0] = "mutated"
	if got := ent.PrimaryKeys()[0]; got != "id" {
		t.Errorf("PrimaryKeys() after mutation = %q, want id", got)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
