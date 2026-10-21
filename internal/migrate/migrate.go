// Package migrate prepares versioned migrations: it loads the consumer's
// embedded resources, inspects the history table, compares the current and
// desired versions, builds the ordered execution plan and evaluates the
// transactional scope per dialect. It does not execute migration DDL; applying
// the plan belongs to E08-P03. Preparation performs read-only inspection
// queries (history and managed-object existence) but runs no migration
// statement; every configuration, association, history and atomicity problem
// is reported before any migration statement runs (E08-P01).
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
	"github.com/mineot/krios/internal/sqlfile"
)

// executor is the Krios execution contract satisfied by *DB and *Tx. It is
// declared here structurally so that migrate does not depend on the access or
// transaction packages; using it preserves the DB/Tx guards and the
// transaction policy of E03 unchanged.
type executor interface {
	ExecContext(context.Context, string, ...any) (result.Result, error)
	QueryContext(context.Context, string, ...any) (*scan.Rows, error)
	QueryRowContext(context.Context, string, ...any) scan.Row
}

// statement is the write-only contract used by the history helpers. It is
// satisfied by both the DB executor and a migration Tx, so a history record
// can be written inside or outside a transaction.
type statement interface {
	ExecContext(context.Context, string, ...any) (result.Result, error)
}

// Tx is the transaction contract used to execute one migration scope. It is
// satisfied structurally by *transaction.Tx; the access layer exposes an
// adapter through Beginner so that migrate never imports the transaction
// package. Commit is first-error-wins in the underlying transaction: when a
// statement failed, Commit rolls back instead of confirming (E03).
type Tx interface {
	// ExecContext executes a statement without returning rows.
	ExecContext(ctx context.Context, query string, args ...any) (result.Result, error)

	// Commit confirms the scope. When a recorded failure blocks it, the
	// transaction is rolled back instead.
	Commit(ctx context.Context) error

	// Rollback discards the scope. It is attempted even after a failure.
	Rollback(ctx context.Context) error
}

// Beginner starts the transaction that scopes a group of migration files
// (E08-P01 §11). It is satisfied structurally by an access adapter over *DB;
// the interface keeps migrate free of the access and transaction imports.
type Beginner interface {
	Begin(ctx context.Context) (Tx, error)
}

// historyTable is the name of the migrations history table (E08-P01 §7).
const historyTable = "migrations"

// Status values recorded in the history table (E08-P01 §7).
const (
	StatusApplied    = "applied"
	StatusFailed     = "failed"
	StatusInProgress = "in_progress"
)

// Direction names used by the history table and the plan.
const (
	DirectionUp   = "up"
	DirectionDown = "down"
)

// Step is one file to execute within a version and direction, with the
// transactional scope decided for it (E08-P01 §11).
type Step struct {
	// Version is the version this file belongs to.
	Version int

	// Direction is "up" or "down".
	Direction string

	// File is the loaded embedded resource.
	File *sqlfile.SQLFile

	// Transactional reports whether the file can run inside a transaction.
	// It is true for PostgreSQL/SQLite; on MySQL it is true only when every
	// statement in the file is transactional.
	Transactional bool
}

// Plan is the ordered set of steps that move the schema from the current
// version to the desired one. An empty Plan means no migration is needed.
type Plan struct {
	// CurrentVersion is the version the inspection found.
	CurrentVersion int

	// TargetVersion is the desired version.
	TargetVersion int

	// Steps are the files to execute, in order, grouped by version and
	// direction. Down steps run from the highest version down to the target.
	Steps []Step
}

// Prepare loads the embedded resources, inspects the history and returns the
// ordered plan or an error. It rejects invalid associations, a database
// without history that already has managed objects, an in_progress/failed
// record (ErrPartialMigration) and a missing direction for the needed path.
// No migration DDL is executed.
func Prepare(ctx context.Context, exec executor, cfg *config.MigrationsConfig, d dialect.Dialect) (*Plan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: migrations are not configured", errs.ErrInvalidConfig)
	}

	if exec == nil {
		return nil, fmt.Errorf("%w: migration preparation has no executor", errs.ErrInvalidQuery)
	}

	if d == nil {
		return nil, fmt.Errorf("%w: migration preparation has no dialect", errs.ErrInvalidQuery)
	}

	migrationDialect, ok := d.(dialect.MigrationDialect)

	if !ok {
		return nil, fmt.Errorf("%w: dialect %s has no migration capabilities", errs.ErrInvalidQuery, d.Name())
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if err := validateManagedObjects(cfg.ManagedObjects, d); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: migrations prepare: %w", err)
	}

	if cfg.TargetVersion == nil {
		return nil, fmt.Errorf("%w: migrations require TargetVersion", errs.ErrInvalidConfig)
	}

	target := *cfg.TargetVersion

	if target < 0 {
		return nil, fmt.Errorf("%w: migration target version must be >= 0", errs.ErrInvalidConfig)
	}

	current, err := currentVersion(ctx, exec, cfg, d)

	if err != nil {
		return nil, err
	}

	plan := &Plan{CurrentVersion: current, TargetVersion: target}

	switch {
	case current == target:
		return plan, nil
	case current < target:
		steps, err := upSteps(cfg, current, target, migrationDialect)

		if err != nil {
			return nil, err
		}

		plan.Steps = steps
	default:
		steps, err := downSteps(cfg, current, target, migrationDialect)

		if err != nil {
			return nil, err
		}

		plan.Steps = steps
	}

	return plan, nil
}

// upSteps builds the up sequence from current+1 up to the target, in declared
// order, loading and classifying every file.
func upSteps(cfg *config.MigrationsConfig, current, target int, md dialect.MigrationDialect) ([]Step, error) {
	var steps []Step

	for _, v := range cfg.Versions {
		if v.Version <= current || v.Version > target {
			continue
		}

		if len(v.Up) == 0 {
			return nil, fmt.Errorf("%w: version %d has no up migration to reach target %d", errs.ErrInvalidQuery, v.Version, target)
		}

		for _, name := range v.Up {
			step, err := loadStep(cfg, v.Version, DirectionUp, name, md)

			if err != nil {
				return nil, err
			}

			steps = append(steps, step)
		}
	}

	if target > 0 {
		maxVer := 0

		for _, v := range cfg.Versions {
			if v.Version > maxVer {
				maxVer = v.Version
			}
		}

		if maxVer < target {
			return nil, fmt.Errorf("%w: cannot reach target version %d from declared versions", errs.ErrInvalidQuery, target)
		}

		// ensure target is declared/coverable in the planned range
		found := false

		for _, v := range cfg.Versions {
			if v.Version == target {
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("%w: cannot reach target version %d from declared versions", errs.ErrInvalidQuery, target)
		}

		if len(steps) == 0 && current < target {
			return nil, fmt.Errorf("%w: cannot reach target version %d from declared versions", errs.ErrInvalidQuery, target)
		}
	}

	return steps, nil
}

// downSteps builds the down sequence from the current version down to the
// target (exclusive), highest version first, using each version's declared
// Down order.
func downSteps(cfg *config.MigrationsConfig, current, target int, md dialect.MigrationDialect) ([]Step, error) {
	var steps []Step

	for i := len(cfg.Versions) - 1; i >= 0; i-- {
		v := cfg.Versions[i]

		if v.Version > current || v.Version <= target {
			continue
		}

		if len(v.Down) == 0 {
			return nil, fmt.Errorf("%w: version %d has no down migration to reach target %d", errs.ErrInvalidQuery, v.Version, target)
		}

		for _, name := range v.Down {
			step, err := loadStep(cfg, v.Version, DirectionDown, name, md)

			if err != nil {
				return nil, err
			}

			steps = append(steps, step)
		}
	}

	if target == 0 && current > 0 {
		// must cover all versions above target; if any missing down -> error above
	} else if target > 0 && current > target {
		found := false

		for _, v := range cfg.Versions {
			if v.Version == target {
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("%w: cannot reach target version %d from declared versions", errs.ErrInvalidQuery, target)
		}

		if len(steps) == 0 && current > target {
			return nil, fmt.Errorf("%w: cannot reach target version %d from declared versions", errs.ErrInvalidQuery, target)
		}
	}

	return steps, nil
}

// loadStep loads the embedded file and decides its transactional scope.
func loadStep(cfg *config.MigrationsConfig, version int, direction, name string, md dialect.MigrationDialect) (Step, error) {
	f, err := sqlfile.NewSQLFile(cfg.FS, name)

	if err != nil {
		return Step{}, err
	}

	transactional, err := fileTransactional(f.Text(), md)

	if err != nil {
		return Step{}, fmt.Errorf("%w: %s: %w", errs.ErrInvalidQuery, name, err)
	}

	return Step{Version: version, Direction: direction, File: f, Transactional: transactional}, nil
}

// fileTransactional reports whether every statement in text is safe to run in
// a transaction for the dialect. Every statement is classified (C03-P02 D12):
// on MySQL a statement outside the transactional allowlist makes the file
// non-transactional ("in doubt, false"); on SQLite/PostgreSQL the boolean
// result is discarded because DDL is transactional by SupportsTransactionalDDL,
// but errors still propagate — transaction control statements are rejected and
// malformed text (unterminated block comment) fails preparation. The file text
// sent to the driver is never rewritten (E07-P01 §9).
func fileTransactional(text string, md dialect.MigrationDialect) (bool, error) {
	for _, statement := range splitStatements(text) {
		ok, err := md.IsTransactionalStatement(statement)

		if err != nil {
			return false, err
		}

		if md.SupportsTransactionalDDL() {
			continue
		}

		if !ok {
			return false, nil
		}
	}

	return true, nil
}

// currentVersion inspects the history to determine the current effective version.
// A missing history table means version 0, and is verified against managed objects.
// Version is effective only when completely applied and not completely reverted.
func currentVersion(ctx context.Context, exec executor, cfg *config.MigrationsConfig, d dialect.Dialect) (int, error) {
	records, exists, err := readHistory(ctx, exec, d)

	if err != nil {
		return 0, err
	}

	if !exists {
		if err := verifyNoManagedObjects(ctx, exec, cfg, d); err != nil {
			return 0, err
		}

		return 0, nil
	}

	target := 0

	if cfg.TargetVersion != nil {
		target = *cfg.TargetVersion
	}

	for _, r := range records {
		if r.Status == StatusInProgress || r.Status == StatusFailed {
			last := lastAppliedComplete(records, cfg)

			return 0, errs.NewPartialMigrationError(
				r.Direction, target, last, r.Filename, r.Status,
				"history has an unresolved migration record", nil,
			)
		}
	}

	// detect partial versions
	if err := detectPartial(records, cfg, target); err != nil {
		return 0, err
	}

	return effectiveVersion(records, cfg), nil
}

// lastAppliedComplete returns the highest version that is completely up-applied.
func lastAppliedComplete(records []record, cfg *config.MigrationsConfig) int {
	last := 0

	for _, v := range cfg.Versions {
		if completeUp(records, v) {
			if v.Version > last {
				last = v.Version
			}
		}
	}

	return last
}

// completeUp reports whether all up files of version v are applied.
func completeUp(records []record, v config.MigrationVersion) bool {
	if len(v.Up) == 0 {
		return false
	}

	for _, name := range v.Up {
		if !hasApplied(records, v.Version, name, DirectionUp) {
			return false
		}
	}

	return true
}

// completeDown reports whether all down files of version v are applied.
func completeDown(records []record, v config.MigrationVersion) bool {
	if len(v.Down) == 0 {
		return false
	}

	for _, name := range v.Down {
		if !hasApplied(records, v.Version, name, DirectionDown) {
			return false
		}
	}

	return true
}

func hasApplied(records []record, version int, filename, direction string) bool {
	for _, r := range records {
		if r.Version == version && r.Filename == filename && r.Direction == direction && r.Status == StatusApplied {
			return true
		}
	}

	return false
}

// effectiveVersion is the highest version completely up-applied and not completely reverted.
func effectiveVersion(records []record, cfg *config.MigrationsConfig) int {
	last := 0

	for _, v := range cfg.Versions {
		if completeUp(records, v) && !completeDown(records, v) {
			if v.Version > last {
				last = v.Version
			}
		}
	}

	return last
}

// detectPartial returns ErrPartialMigration if any version is partially applied.
func detectPartial(records []record, cfg *config.MigrationsConfig, target int) error {
	for _, v := range cfg.Versions {
		// up partial
		upApplied := 0
		upTotal := len(v.Up)

		for _, name := range v.Up {
			if hasApplied(records, v.Version, name, DirectionUp) {
				upApplied++
			}
		}

		if upTotal > 0 && upApplied > 0 && upApplied < upTotal {
			// find last applied file
			lastFile := ""

			for _, name := range v.Up {
				if hasApplied(records, v.Version, name, DirectionUp) {
					lastFile = name
				} else {
					break
				}
			}

			lastVer := lastAppliedComplete(records, cfg)
			return errs.NewPartialMigrationError(DirectionUp, target, lastVer, lastFile, StatusInProgress, "version partially applied", nil)
		}

		// down partial
		downApplied := 0
		downTotal := len(v.Down)

		for _, name := range v.Down {
			if hasApplied(records, v.Version, name, DirectionDown) {
				downApplied++
			}
		}

		if downTotal > 0 && downApplied > 0 && downApplied < downTotal {
			lastFile := ""

			for _, name := range v.Down {
				if hasApplied(records, v.Version, name, DirectionDown) {
					lastFile = name
				} else {
					break
				}
			}

			lastVer := lastAppliedComplete(records, cfg)
			return errs.NewPartialMigrationError(DirectionDown, target, lastVer, lastFile, StatusInProgress, "version partially reverted", nil)
		}
	}
	return nil
}

// record is one row of the history table.
type record struct {
	Version   int
	Filename  string
	Direction string
	Status    string
}

// readHistory reports the history records and whether the history table
// exists. A missing table is not an error: it means a schema without history.
func readHistory(ctx context.Context, exec executor, d dialect.Dialect) ([]record, bool, error) {
	query := fmt.Sprintf("SELECT version, filename, direction, status FROM %s", d.QuoteIdent(historyTable))

	rows, err := exec.QueryContext(ctx, query)

	if err != nil {
		if isMissingTable(err) {
			return nil, false, nil
		}

		return nil, false, err
	}

	defer rows.Close()

	var records []record

	for rows.Next() {
		var r record

		if err := rows.Scan(&r.Version, &r.Filename, &r.Direction, &r.Status); err != nil {
			return nil, false, err
		}

		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	return records, true, nil
}

// verifyNoManagedObjects fails when any managed object already exists in a
// database without history (E08-P01 §6).
func verifyNoManagedObjects(ctx context.Context, exec executor, cfg *config.MigrationsConfig, d dialect.Dialect) error {
	for _, object := range cfg.ManagedObjects {
		present, err := objectExists(ctx, exec, object, d)

		if err != nil {
			return err
		}

		if present {
			return fmt.Errorf("%w: database has no migration history but object %q already exists", errs.ErrInvalidQuery, object)
		}
	}

	return nil
}

// objectExists reports whether the named table exists.
func objectExists(ctx context.Context, exec executor, object string, d dialect.Dialect) (bool, error) {
	query, args := existenceQuery(object, d)

	var n int

	err := exec.QueryRowContext(ctx, query, args...).Scan(&n)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("krios: migrations: inspect managed object %q: %w", object, err)
	}

	return n > 0, nil
}

// validateManagedObjects rejects qualified object names on SQLite, where user
// schemas do not exist (C03-P01 D6). Structural checks (empty, padding, at
// most two parts) live in config.MigrationsConfig.Validate.
func validateManagedObjects(objects []string, d dialect.Dialect) error {
	if d.Name() != "sqlite" {
		return nil
	}

	for _, object := range objects {
		if strings.Contains(object, ".") {
			return fmt.Errorf("%w: sqlite has no user schemas; managed object %q must be a plain table name", errs.ErrInvalidConfig, object)
		}
	}

	return nil
}

// existenceQuery returns a dialect-specific query and its arguments that yield
// a positive count when the table exists. The consumer-supplied name is never
// interpolated into the SQL text: it is bound as an argument (C03-P01 D7).
// Simple names are scoped to the schema of the connection — DATABASE() on
// MySQL, current_schema() on PostgreSQL, the single schema of SQLite — so a
// same-named table in another schema is not reported. Qualified "schema.table"
// names target the declared schema on MySQL/PostgreSQL. The query never
// enumerates the whole schema (E08-P01 §6).
func existenceQuery(object string, d dialect.Dialect) (string, []any) {
	schema, table, qualified := strings.Cut(object, ".")

	if !qualified {
		table = object
	}

	switch d.Name() {
	case "sqlite":
		query := fmt.Sprintf("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = %s", d.Placeholder(1))

		return query, []any{table}
	case "mysql":
		if qualified {
			query := fmt.Sprintf(
				"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = %s AND table_name = %s",
				d.Placeholder(1), d.Placeholder(2),
			)

			return query, []any{schema, table}
		}

		query := fmt.Sprintf(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = %s",
			d.Placeholder(1),
		)

		return query, []any{table}
	default:
		if qualified {
			query := fmt.Sprintf(
				"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = %s AND table_name = %s",
				d.Placeholder(1), d.Placeholder(2),
			)

			return query, []any{schema, table}
		}

		query := fmt.Sprintf(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = %s",
			d.Placeholder(1),
		)

		return query, []any{table}
	}
}

// isMissingTable reports whether err indicates the history table is absent.
// The check is textual because database/sql exposes no portable error codes;
// the patterns below are the missing-table phrasings of the supported engines
// and are used only for the history probe (C03-P01 D8): SQLite reports "no
// such table", MySQL "doesn't exist" and PostgreSQL "does not exist" (with
// "undefined table" as a conservative variation). Errors that do not match —
// permissions, connection loss, malformed history — are never treated as a
// missing table and surface to the caller unchanged.
func isMissingTable(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "undefined table")
}

// splitStatements splits text on semicolons that terminate a statement,
// ignoring semicolons inside single/double-quoted strings, backticks and
// line/block comments. It is a conservative splitter used only to classify
// each statement; the file text sent to the driver is never rewritten
// (E07-P01 §9). Unterminated quotes/comments keep the remainder as one
// fragment, which the dialect classifier then rejects as needed.
func splitStatements(text string) []string {
	var (
		statements []string
		current    strings.Builder
		inSingle   bool
		inDouble   bool
		inBacktick bool
		inLine     bool
		inBlock    bool
	)

	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		c := runes[i]

		switch {
		case inLine:
			current.WriteRune(c)

			if c == '\n' {
				inLine = false
			}

			continue
		case inBlock:
			current.WriteRune(c)

			if c == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				current.WriteRune('/')
				i++
				inBlock = false
			}

			continue
		case inSingle:
			current.WriteRune(c)

			if c == '\'' {
				if i+1 < len(runes) && runes[i+1] == '\'' {
					current.WriteRune('\'')
					i++
					continue
				}

				inSingle = false
			}

			continue
		case inDouble:
			current.WriteRune(c)

			if c == '"' {
				inDouble = false
			}

			continue
		case inBacktick:
			current.WriteRune(c)

			if c == '`' {
				inBacktick = false
			}

			continue
		}

		switch c {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '`':
			inBacktick = true
		case '-':
			if i+1 < len(runes) && runes[i+1] == '-' {
				current.WriteString("--")
				i++
				inLine = true
				continue
			}
		case '/':
			if i+1 < len(runes) && runes[i+1] == '*' {
				current.WriteString("/*")
				i++
				inBlock = true
				continue
			}
		case ';':
			if s := strings.TrimSpace(current.String()); s != "" {
				statements = append(statements, s)
			}

			current.Reset()
			continue
		}

		current.WriteRune(c)
	}

	if s := strings.TrimSpace(current.String()); s != "" {
		statements = append(statements, s)
	}

	return statements
}
