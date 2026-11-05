package dialects

import "testing"

func TestDialects(t *testing.T) {
	for _, tc := range []struct {
		dialect              Dialect
		quoted, first, tenth string
	}{
		{PostgreSQL, `"a""b` + "`" + `c"`, "$1", "$10"},
		{SQLite3, `"a""b` + "`" + `c"`, "?", "?"},
		{MySQL, "`a\"b``c`", "?", "?"},
	} {
		t.Run(string(tc.dialect), func(t *testing.T) {
			d, ok := Lookup(tc.dialect)

			if !ok {
				t.Fatal("missing dialect")
			}

			if got := d.QuoteIdentifier("a\"b`c"); got != tc.quoted {
				t.Fatalf("quote = %q, want %q", got, tc.quoted)
			}

			if d.Placeholder(1) != tc.first || d.Placeholder(10) != tc.tenth {
				t.Fatal("incorrect placeholders")
			}
		})
	}

	for _, value := range []Dialect{"", "POSTGRES", "secret"} {
		if d, ok := Lookup(value); ok || d != nil {
			t.Fatal("accepted unsupported dialect")
		}
	}
}
