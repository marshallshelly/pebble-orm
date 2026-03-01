# Changelog

All notable changes to Pebble ORM will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.16.3] - 2026-03-01

### Fixed

- `CREATE TABLE` statements in generated migrations now respect foreign key dependencies: referenced tables are always emitted before the tables that reference them. Previously, tables could appear in source-file order, causing FK constraint violations when applying migrations to PostgreSQL.

## [1.16.2] - 2026-03-01

### Fixed

- Foreign key constraints missing from all generated migrations. Three separate bugs:
  - AST loader (`pkg/loader`) never parsed `fk:table(column)` tag options — `ForeignKeys` was always empty.
  - Runtime parser (`pkg/schema`) read from `"db"` struct tag instead of `"po"` in `parseForeignKeys`.
  - Schema reconstructor (`pkg/migration`) did not parse `CONSTRAINT … FOREIGN KEY … REFERENCES` from existing migration SQL, causing all FKs to appear as newly added on every subsequent `pebble generate` run.
- Incremental `pebble generate` now emits only genuinely new tables/columns when FK constraints are present, with no spurious `ALTER TABLE ADD CONSTRAINT` for already-migrated tables.

### Changed

- `strings.IndexByte` + manual slice arithmetic replaced with `strings.Cut` in column definition parser.
- Untagged `switch` statements on character values converted to tagged `switch` in `typeTokenEnd` and index-definition tokenizer.

## [1.16.1] - 2026-02-23

### Fixed

- `pebble diff` build failure: dropped tables, columns, and indexes were passed as struct types to `%s` format verbs instead of their `.Name` fields.

## [1.16.0] - 2026-02-23

### Added

- Schema reconstruction from existing migration files as the offline baseline for `pebble generate`, eliminating the need for a separate schema snapshot JSON file. The CLI now replays all up-migration files in chronological order to determine the current schema state without a database connection.

### Fixed

- Down migrations now generate complete, executable SQL for all dropped items instead of placeholder comments:
  - Dropped tables → full `CREATE TABLE` statement reconstructed from original schema
  - Dropped columns → `ALTER TABLE ... ADD COLUMN` with original type and constraints
  - Dropped indexes → full `CREATE INDEX` with original definition
  - Dropped foreign keys → `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY`
  - Dropped constraints → `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE`
  - Dropped enum types → `CREATE TYPE ... AS ENUM` with original values
- Enum type modification down migrations now correctly describe the PostgreSQL limitation (values cannot be removed) rather than emitting a misleading TODO comment.

## [1.15.1] - 2026-02-16

### Fixed

- AST loader missing UNIQUE constraint metadata, causing spurious `DROP CONSTRAINT` in diff migrations.
- `pg_advisory_lock()` scan error from scanning `void` return into `*bool` in migration executor.

### Added

- Loader now generates `ConstraintMetadata` for `unique` columns, matching the reflection-based parser.
- Tests for UNIQUE constraint generation in AST loader.

## [1.15.0] - 2026-02-13

### Changed

- **BREAKING**: Minimum Go version bumped from 1.24 to 1.26.
- Modernized struct field iteration to use Go 1.26 `reflect.Type.Fields()` iterators in `parser.go` and `relationships.go`.
- Users benefit from Go 1.26 runtime improvements: Green Tea GC (10-40% lower GC overhead), faster `fmt.Errorf` allocations, improved slice stack allocation, and `io.ReadAll` performance.

## [1.14.6] - 2026-01-30

### Added

- Custom array types for PgBouncer/simple_protocol compatibility: `StringArray`, `Int32Array`, `Int64Array`, `Float64Array`, `BoolArray`.
- PostgreSQL text format array parsing for `{value1,value2,value3}` syntax.
- Full support for quoted elements, escaped characters, and NULL values in array parsing.
- Comprehensive test suite for array types (50+ test cases).

## [1.14.5] - 2026-01-25

### Fixed

- JSONB type incorrectly mapped to `json` instead of `jsonb` in CLI-generated migrations.
- TimestampTZ type incorrectly mapped to `timestamp` instead of `timestamptz` in CLI migrations.
- BigSerial type incorrectly mapped to `serial` instead of `bigserial` in CLI migrations.

### Added

- Comprehensive test suite for loader package SQL type detection.
- Missing PostgreSQL types `double precision` and `interval` to loader type list.

## [1.14.4] - 2026-01-13

### Changed

- Removed debug logging from `introspector.go` after verifying index recreation bug fix.

## [1.14.3] - 2026-01-13

### Fixed

- Index introspection failing when PostgreSQL returns schema-qualified table names.
- Empty `Columns` arrays in introspected indexes causing unnecessary index recreation.

## [1.14.1] - 2026-01-13

### Fixed

- Schema differ incorrectly detecting indexes as different on every deployment.
- Empty column orderings compared as different from explicit ASC orderings.

## [1.14.0] - 2026-01-12

### Added

- Nested preload support using dot notation: `Preload("Client.Route")`.
- Deep nesting support: `Preload("Author.Profile.Avatar")`.
- Efficient batched loading for nested relationships.
- `loadNestedRelationships()` for recursive relationship loading.
- `loadRelationshipOnCollection()` for loading on arbitrary object collections.

## [1.13.2] - 2026-01-10

### Fixed

- JSONB auto-marshaling now correctly returns `string` for pgx encoding.

## [1.13.1] - 2026-01-09

### Fixed

- Introspector `int2vector` scan error when fetching index metadata.
- JSONB auto-marshaling returns string instead of `[]byte` for proper pgx encoding.

## [1.13.0] - 2026-01-09

### Added

- Native JSONB auto-marshaling for custom types without `driver.Valuer`/`sql.Scanner`.
- `IsJSONB` flag on `ColumnMetadata` for automatic serialization tracking.
- JSONB column detection from `jsonb`/`json` tag options or SQL type.
- `marshalJSONB()` helper for automatic JSON serialization on insert/update.
- `jsonbScanTarget` scanner for automatic JSON deserialization on select.
- Backward compatibility with types implementing `Value()`/`Scan()`.

## [1.12.0] - 2026-01-08

### Added

- Comprehensive PostgreSQL index support with column-level and table-level syntax.
- Column-level index tags: `index`, `index(name)`, `index(name,type)`, `index(name,type,desc)`.
- Table-level index comments for complex indexes with full PostgreSQL syntax.
- Expression indexes, partial indexes, and covering indexes support.
- Operator classes and collations for indexes.
- CONCURRENTLY flag for production-safe index creation.
- Full index introspection from existing databases.
- Index modification detection in schema differ.
- 148 comprehensive tests for index functionality.

## [1.11.0] - 2026-01-04

### Added

- Generic transaction functions: `TxSelect[T]`, `TxInsert[T]`, `TxUpdate[T]`, `TxDelete[T]`.
- Enhanced migration generation output with visual indicators.
- Row locking examples using `ForUpdate()` in transaction documentation.
- Savepoints example demonstrating nested transaction control.

### Fixed

- Transaction query builders completely broken due to incomplete `scanRows()`.
- JSONB scanning errors when pgx passes pre-decoded values.
- Transaction method signatures now return proper types.

### Changed

- Rewrote transaction examples to use type-safe query builders.
- Migration generation now always shows model scanning results.

## [1.10.0] - 2026-01-03

### Added

- Native JSONB struct scanning without wrapper types using pgx v5's `JSONBCodec`.
- Direct struct field support for JSONB columns.
- Full NULL handling via pointer types for JSONB fields.
- Support for any JSON-compatible Go type in JSONB columns.
- Comprehensive JSONB test suite and examples.

### Changed

- Updated documentation with JSONB examples showing three supported approaches.

## [1.9.1] - 2026-01-02

### Fixed

- Migration indeterminacy from `decimal`/`numeric` and `time` type mismatches.
- `pebble generate metadata` output now deterministic with sorted table names.
- Migration example argument order for proper diff generation direction.

## [1.9.0] - 2026-01-02

### Fixed

- PostgreSQL reserved keywords causing syntax errors in DROP TABLE statements.

### Added

- `quoteIdent()` helper function for safe identifier quoting.

## [1.8.9] - 2026-01-02

### Fixed

- Prepared statement caching errors in schema introspection on retry.
- Extended `QueryExecModeExec` fix to all introspector queries.

## [1.8.8] - 2026-01-01

### Fixed

- Prepared statement caching causing migration idempotency failures.
- Migration executor now uses `QueryExecModeExec` to bypass statement cache.

## [1.8.7] - 2025-12-31

### Added

- PostgreSQL ENUM type support with `enum(value1,value2,...)` tag syntax.
- Automatic `CREATE TYPE ... AS ENUM` generation in migrations.
- Automatic `ALTER TYPE ... ADD VALUE` for new enum values.
- Enum type introspection via `pg_enum` system catalog.
- Smart enum deduplication for multiple columns using same type.

## [1.8.6] - 2025-12-31

### Fixed

- pgx binary format scan error in constraint introspection for `char` type.
- UNIQUE constraint auto-migration now works correctly.

## [1.8.5] - 2025-12-31

### Added

- UNIQUE constraint auto-migration support with `unique` struct tags.
- Automatic `ALTER TABLE ADD CONSTRAINT ... UNIQUE` SQL generation.
- Single-column and composite UNIQUE constraint support.
- Smart constraint comparison by columns instead of names.

### Fixed

- UNIQUE constraints completely ignored during auto-migration.
- Introspector now detects UNIQUE constraints (`contype = 'u'`).
- Migration differ compares UNIQUE constraints using column-based keys.

## [1.8.4] - 2025-12-30

### Fixed

- Preload failing with pgx encode error for `ANY($1)` queries.
- Added `convertToTypedSlice()` helper for proper array parameter encoding.

## [1.8.3] - 2025-12-30

### Fixed

- Preload failing when foreign keys are nullable pointers.
- Added pointer dereferencing before collecting key values in relationship loading.

## [1.8.2] - 2025-12-30

### Fixed

- Preload operations broken due to Go initialism handling in `toPascalCase()`.
- Rewrote `toPascalCase()` to recognize 38 common Go initialisms (ID, API, URL, etc.).

## [1.8.1] - 2025-12-30

### Fixed

- Preload failing with "table not registered" for custom table names.
- Added `TargetType` field to `RelationshipMetadata` for accurate registry lookup.

## [1.8.0] - 2025-12-29

### Added

- Smart default value detection for automatic zero-value omission on insert.
- Fields with `default()` tag and zero value are automatically omitted.
- Identity columns also omitted when zero-valued.

## [1.7.2] - 2025-12-29

### Fixed

- Migration planner generating DROP INDEX for constraint-backed indexes.
- Type conversions without USING clauses blocking schema evolution.

### Added

- Automatic USING clause generation for common type conversions.
- `PRODUCTION_SAFETY.md` documenting migration best practices.
- `requiresUsingClause()` and `generateUsingClause()` helpers.

## [1.7.1] - 2025-12-28

### Fixed

- Phantom ALTER commands from type normalization differences.
- Duplicate UNIQUE indexes for columns with `unique` tag.
- Identity columns not parsed in CLI `pebble generate` command.
- Identity SQL generation syntax errors.

### Added

- PostgreSQL identity columns with `identity`, `identityAlways`, `identityByDefault` tags.
- `GENERATED ALWAYS AS IDENTITY` and `GENERATED BY DEFAULT AS IDENTITY` support.

### Changed

- `CREATE TABLE` uses inline `PRIMARY KEY` for single-column PKs.
- Replaced deprecated `reflect.PtrTo` with `reflect.PointerTo`.

## [1.6.1] - 2025-12-27

### Fixed

- Auto-migration generating DROP DEFAULT for serial columns.
- Added serial column awareness to default comparison.

## [1.6.0] - 2025-12-27

### Added

- CLI migration generation from Go source files without database connection.
- `pebble generate --name migration_name --models ./path/to/models` command.
- AST-based schema building in new loader package.
- `Registry.RegisterMetadata()` for direct metadata registration.

### Changed

- `--db` flag now optional for initial migrations.

## [1.5.3] - 2025-12-23

### Fixed

- Migration planner generating invalid `ALTER COLUMN TYPE serial` SQL.
- Serial types now mapped to underlying base types in comparisons.

## [1.5.2] - 2025-12-23

### Added

- `pebble generate metadata --scan ./path` for production-safe table name generation.
- Generated `table_names.gen.go` with compile-time registrations.

### Fixed

- Custom table names failing in production Docker builds.

## [1.5.1] - 2025-12-23

### Fixed

- `// table_name:` comment directives not being parsed.
- `findSourceFile()` now searches current working directory first.

## [1.5.0] - 2025-12-23

### Added

- PostgreSQL generated columns with `GENERATED ALWAYS AS` support.
- `STORED` generated columns computed on INSERT/UPDATE.
- Tag syntax: `po:"column_name,generated:EXPRESSION,stored"`.

## [1.4.0] - 2025-12-23

### Added

- Safe auto-migrations with `IF NOT EXISTS` by default.
- `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`.
- `PlannerOptions` struct for migration configuration.
- `NewPlannerWithOptions()` for custom migration behavior.

## [1.3.1] - 2025-12-22

### Changed

- Updated `transactions` and `relationships` examples to use `builder.Col`.

## [1.3.0] - 2025-12-22

### Added

- Type-safe column names with `builder.Col[T](fieldName)` helper.
- pkg.go.dev documentation badge.

### Changed

- Foreign key tags now use camelCase: `onDelete:` and `onUpdate:`.
- Integration tests use ORM's migration system.

### Fixed

- Tag naming inconsistency across foreign key constraints.
- GoReleaser deprecation warnings.

## [1.2.1] - 2025-12-22

### Added

- `builder.Col[T](fieldName)` helper for type-safe column names.
- pkg.go.dev badge to README.

### Changed

- Integration tests now use Pebble ORM's migration system.
- Updated all examples to use `builder.Select[T](qb)` pattern.

### Fixed

- GoReleaser configuration deprecation warnings.
- Integration test compilation with new builder API.

## [1.1.0] - 2025-01-XX

### Changed

- Project adheres to Semantic Versioning.

## [1.0.0] - 2025-12-22

### Added

- Type-safe query builder using Go 1.21+ generics.
- Zero-overhead performance with native pgx integration.
- Struct-tag based schema definitions.
- Automatic migration generation and management.
- Full relationship support (hasMany, hasOne, belongsTo, manyToMany).
- CASCADE DELETE via foreign key constraint tags.
- Transaction support with proper error handling.
- JSONB support for flexible JSON data.
- Array types for multi-value columns.
- UUID primary key support.
- Geometric types (point, polygon, circle, etc.).
- Full-text search capabilities.
- Interactive CLI with TUI for migrations and introspection.
- 7 comprehensive examples with production-ready structure.
- Integration tests with testcontainers.
- GitHub Actions CI workflow.
- golangci-lint integration.
- GoReleaser configuration for multi-platform releases.

[unreleased]: https://github.com/marshallshelly/pebble-orm/compare/v1.16.3...HEAD
[1.16.3]: https://github.com/marshallshelly/pebble-orm/compare/v1.16.2...v1.16.3
[1.16.2]: https://github.com/marshallshelly/pebble-orm/compare/v1.16.1...v1.16.2
[1.16.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.16.0...v1.16.1
[1.16.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.15.1...v1.16.0
[1.15.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.15.0...v1.15.1
[1.15.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.14.6...v1.15.0
[1.14.6]: https://github.com/marshallshelly/pebble-orm/compare/v1.14.5...v1.14.6
[1.14.5]: https://github.com/marshallshelly/pebble-orm/compare/v1.14.4...v1.14.5
[1.14.4]: https://github.com/marshallshelly/pebble-orm/compare/v1.14.3...v1.14.4
[1.14.3]: https://github.com/marshallshelly/pebble-orm/compare/v1.14.1...v1.14.3
[1.14.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.14.0...v1.14.1
[1.14.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.13.2...v1.14.0
[1.13.2]: https://github.com/marshallshelly/pebble-orm/compare/v1.13.1...v1.13.2
[1.13.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.13.0...v1.13.1
[1.13.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.12.0...v1.13.0
[1.12.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.11.0...v1.12.0
[1.11.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.10.0...v1.11.0
[1.10.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.9.1...v1.10.0
[1.9.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.9.0...v1.9.1
[1.9.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.9...v1.9.0
[1.8.9]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.8...v1.8.9
[1.8.8]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.7...v1.8.8
[1.8.7]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.6...v1.8.7
[1.8.6]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.5...v1.8.6
[1.8.5]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.4...v1.8.5
[1.8.4]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.3...v1.8.4
[1.8.3]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.2...v1.8.3
[1.8.2]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.1...v1.8.2
[1.8.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.8.0...v1.8.1
[1.8.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.7.2...v1.8.0
[1.7.2]: https://github.com/marshallshelly/pebble-orm/compare/v1.7.1...v1.7.2
[1.7.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.6.1...v1.7.1
[1.6.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.6.0...v1.6.1
[1.6.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.5.3...v1.6.0
[1.5.3]: https://github.com/marshallshelly/pebble-orm/compare/v1.5.2...v1.5.3
[1.5.2]: https://github.com/marshallshelly/pebble-orm/compare/v1.5.1...v1.5.2
[1.5.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.5.0...v1.5.1
[1.5.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.3.1...v1.4.0
[1.3.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.2.1...v1.3.0
[1.2.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.1.0...v1.2.1
[1.1.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/marshallshelly/pebble-orm/releases/tag/v1.0.0
