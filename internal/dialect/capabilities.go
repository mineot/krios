package dialect

// Capabilities describes known native syntax, not permissions or object-level support.
type Capabilities struct {
	FullJoin          bool
	ExplicitNullOrder bool
	Returning         bool
}

// Info is an immutable initialization snapshot retained by a connection.
type Info struct {
	Dialect      Dialect
	Version      Version
	Capabilities Capabilities
}

func capabilities(kind Dialect, version Version) Capabilities {
	switch kind {
	case Postgres:
		if version.atLeast(Version{Major: 16}) {
			return Capabilities{FullJoin: true, ExplicitNullOrder: true, Returning: true}
		}
	case SQLite:
		if version.atLeast(Version{Major: 3, Minor: 39}) {
			return Capabilities{FullJoin: true, ExplicitNullOrder: true, Returning: true}
		}
	}

	// MySQL has none of these native clauses in the supported baseline.
	// Do not invent capabilities when an unknown newer version is encountered.
	return Capabilities{}
}
