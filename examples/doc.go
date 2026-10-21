// Package examples contains KriosTasks, a small task manager written against
// the public Krios API. It shows how a consumer project configures database
// access, owns its schema with embedded SQL, runs explicit transactions, builds
// chained SELECT queries, maps query results into structs and applies per-field
// validation and formatting.
//
// The package covers the deliveries implemented so far (E01–E08): connection
// configuration and lifecycle, SQL execution, explicit transactions, entity
// mapping with validation and formatting, the SELECT builder with filters,
// joins, aggregates and pagination, entity-scoped INSERT/UPDATE/DELETE
// builders with value policy, mandatory filters and RETURNING support per
// database, and execution of SQL files embedded with go:embed (see the schema/
// directory). Versioned migrations (E08) are configured through
// Config.Migrations and run with DB.RunMigrations; the migration example in
// migrations.go owns a second, separate schema (projects and project_members)
// in the same database file, so the two schema owners never share a table.
//
// Run executes the full narrative against a SQLite file and prints each step.
// Migrate runs the versioned schema through an advance, a revert and a
// reapplication, printing the plan of each phase. The MySQL and PostgreSQL
// configurations in config.go differ only in the driver import and the Config
// value; statement placeholders follow the dialect because consumer SQL is
// never translated between engines. The SELECT and write builders render the
// dialect's placeholders automatically.
package examples
