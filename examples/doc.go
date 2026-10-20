// Package examples contains KriosTasks, a small task manager written against
// the public Krios API. It shows how a consumer project configures database
// access, owns its schema with raw SQL, runs explicit transactions, builds
// chained SELECT queries, maps query results into structs and applies per-field
// validation and formatting.
//
// The package covers the deliveries implemented so far (E01–E05): connection
// configuration and lifecycle, SQL execution, explicit transactions, entity
// mapping with validation and formatting, and the SELECT builder with filters,
// joins, aggregates and pagination. Write builders, embedded SQL execution and
// migrations (E06–E08) are not used yet, so schema and write statements remain
// consumer-owned SQL.
//
// Run executes the full narrative against a SQLite file and prints each step.
// The MySQL and PostgreSQL configurations in config.go differ only in the
// driver import and the Config value; statement placeholders follow the
// dialect because consumer SQL is never translated between engines. The
// SELECT builder renders the dialect's placeholders automatically.
package examples
