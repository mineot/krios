package dialect

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mineot/krios/internal/errs"
)

// Version contains numeric components used for compatibility checks.
// PostgreSQL 10+ release numbers use Major and Patch, with Minor set to zero.
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) atLeast(minimum Version) bool {
	if v.Major != minimum.Major {
		return v.Major > minimum.Major
	}

	if v.Minor != minimum.Minor {
		return v.Minor > minimum.Minor
	}

	return v.Patch >= minimum.Patch
}

func parseVersion(kind Dialect, raw string) (Version, error) {
	raw = strings.TrimSpace(raw)

	if kind == Postgres {
		n, err := decimal(raw)

		if err != nil {
			return Version{}, err
		}

		if n < 10000 {
			return Version{}, fmt.Errorf("invalid PostgreSQL version number")
		}

		if n >= 100000 {
			return Version{Major: n / 10000, Patch: n % 10000}, nil
		}

		return Version{Major: n / 10000, Minor: n / 100 % 100, Patch: n % 100}, nil
	}

	if kind == MySQL {
		// MariaDB uses a separate release series and is not a supported dialect.
		if strings.Contains(strings.ToLower(raw), "mariadb") {
			return Version{}, fmt.Errorf("%w: MariaDB is not MySQL", errs.ErrUnsupported)
		}

		// MySQL may append a distribution suffix to its numeric version.
		if index := strings.IndexAny(raw, "-+"); index >= 0 {
			raw = raw[:index]
		}
	}

	parts := strings.Split(raw, ".")

	if len(parts) != 3 {
		return Version{}, fmt.Errorf("expected three numeric version components")
	}

	var components [3]int

	for i, part := range parts {
		n, err := decimal(part)

		if err != nil {
			return Version{}, err
		}

		components[i] = n
	}

	return Version{Major: components[0], Minor: components[1], Patch: components[2]}, nil
}

func decimal(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("empty version component")
	}

	for _, c := range value {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-numeric version component")
		}
	}

	n, err := strconv.Atoi(value)

	if err != nil {
		// Do not include the raw server response in a library-generated error.
		return 0, fmt.Errorf("version component exceeds integer range")
	}

	return n, nil
}

func validateVersion(kind Dialect, version Version) error {
	var minimum Version

	switch kind {
	case Postgres:
		minimum = Version{Major: 16}
	case MySQL:
		minimum = Version{Major: 8}
	case SQLite:
		minimum = Version{Major: 3, Minor: 39}
	default:
		return fmt.Errorf("%w: unknown dialect", errs.ErrInvalidConfig)
	}

	if !version.atLeast(minimum) {
		return fmt.Errorf("%w: database is below the minimum supported version", errs.ErrUnsupported)
	}

	return nil
}
