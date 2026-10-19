// Package examples contains KriosTasks, a small task manager written against
// the public Krios API. It shows how a consumer project configures database
// access, owns its schema with raw SQL, runs explicit transactions, maps query
// results into structs and applies per-field validation and formatting.
//
// The package covers the deliveries implemented so far (E01–E04): connection
// configuration and lifecycle, SQL execution, explicit transactions and
// entity mapping with validation and formatting. Query builders, write
// builders, embedded SQL execution and migrations (E05–E08) are not used yet,
// so schema and statements are consumer-owned SQL.
//
// Run executes the full narrative against a SQLite file and prints each step.
// The MySQL and PostgreSQL configurations in config.go differ only in the
// driver import and the Config value; statement placeholders follow the
// dialect because consumer SQL is never translated between engines.
package examples
