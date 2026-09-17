# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.27.0] - 2026-09-17

### Added

- `--sequential` flag on `pebble generate`, numbering migrations
  `000_name.up.sql`, `001_...`, `002_...` instead of by timestamp.
- Error when a migrations directory mixes timestamp and sequential versions.
- Error when a sequential version outgrows its zero-padding, since `1000` sorts
  before `999`.

### Changed

- A migrations directory keeps the version scheme it already uses, so
  `--sequential` applies only to the first migration in a directory.
- `pkg/builder` uses `strings.Builder`, `slices.Contains`, `maps.Copy`, and
  `reflect.TypeFor` in place of hand-written equivalents.

### Fixed

- README documented generated migrations as `0001_name.up.sql`. The generator
  has always used timestamps.

## [1.26.0] - 2026-08-21

### Changed

- **BREAKING**: Minimum Go version bumped from 1.26 to 1.27.
- Upgrade `github.com/charmbracelet/bubbles` from v0.21.0 to v1.0.0.
- Upgrade `github.com/stretchr/testify` from v1.11.1 to v1.12.1.
- Upgrade `github.com/testcontainers/testcontainers-go` from v0.40.0 to v0.44.0.

## [1.25.1] - 2026-08-06

### Fixed

- Index column modifiers are emitted in the order PostgreSQL requires, `COLLATE`
  before the operator class. The previous order was a syntax error when a column
  had both.
- `autoIncrement` columns map to the matching serial type, so PostgreSQL creates
  the backing sequence. They were emitted as plain integers with no default, and
  inserts relying on a generated key failed with a NOT NULL violation.
- Generated columns are omitted from generated `INSERT` statements. A value can
  never be inserted into a `GENERATED ALWAYS` column (SQLSTATE 428C9).
- A migration containing `CREATE INDEX CONCURRENTLY` is applied in autocommit
  mode rather than a transaction block, which PostgreSQL forbids. Such a
  migration is not atomic.
- Expression indexes and partial-index predicates no longer phantom-diff.
  PostgreSQL rewrites the SQL it stores, so the differ canonicalizes both the
  authored and stored forms before comparing.
- Array columns no longer phantom-diff. The type normalizer normalizes the
  element type inside an array, so an authored `integer[]` matches the
  introspected `int4[]`.

## [1.25.0] - 2026-08-06

### Added

- Composite UNIQUE constraints through a table-level
  `// unique: <name> (col1, col2, ...)` directive, the natural fit for junction
  tables. Single-column uniqueness continues to use the `unique` column tag.
- Composite UNIQUE parsing on both the reflection and AST paths, emitted in
  `CREATE TABLE` and `ALTER TABLE`, introspected from `pg_constraint`, and
  reconstructed from existing migration files.

### Fixed

- Constraint introspection returns a constraint's columns in their declared
  order rather than table-column order, so a composite UNIQUE whose column order
  differs from the table's no longer phantom-diffs.

## [1.24.1] - 2026-08-06

### Fixed

- Partial indexes no longer phantom-diff. The differ normalizes a `WHERE`
  predicate before comparing, so an authored `WHERE read = false` matches
  PostgreSQL's stored `WHERE (read = false)`.
- A modified index emits `DROP INDEX` before `CREATE INDEX` in the up migration.
  The order was reversed, so the recreated index was immediately dropped.
- The offline reconstruct replays `CREATE INDEX`, `DROP INDEX`, and
  `ALTER COLUMN ... SET DEFAULT` from existing migration files. These were
  ignored, so every offline `generate` re-emitted the same change.

## [1.24.0] - 2026-08-04

### Added

- DOMAIN types through a table-level
  `// domain: <name> AS <base type> [CHECK (...)]` directive, adopted by a
  column with a `domain(<name>)` tag.
- `CREATE DOMAIN` emitted ahead of the tables that use it, parsed on both the
  reflection and AST paths.
- Domain introspection from `pg_type`, reconstruction from existing migration
  files, and add/drop diffing by name.
- Domain-typed columns introspected by their domain name rather than the
  underlying base type, so incremental `pebble generate` produces no phantom
  type change.

## [1.23.0] - 2026-08-04

### Added

- Extension management through a table-level `// extension: <name>` directive,
  parsed on both the reflection and AST paths.
- `CREATE EXTENSION IF NOT EXISTS "<name>"` emitted ahead of any enum type or
  table.
- Extension introspection from `pg_extension` and reconstruction from existing
  migration files, so incremental runs do not re-add an installed extension.
- Extension diffing is add-only. An extension is never dropped, since a database
  carries system extensions Pebble does not manage.

## [1.22.0] - 2026-08-03

### Added

- Column-level `check(expr)` tag, generating a named
  `CONSTRAINT {table}_{column}_check`.
- Table-level `// check: <name> <expr>` directive for multi-column CHECK
  constraints.
- CHECK parsing on both the reflection and AST paths, emitted in `CREATE TABLE`
  and `ALTER TABLE`, and reconstructed from existing migration files.
- Introspected CHECK constraints normalized to match the authored form, so
  incremental `pebble generate` no longer drops a database CHECK it can express.
- Expressions containing commas and quoted values are preserved, such as
  `check(status IN ('active', 'closed'))`.

## [1.21.0] - 2026-08-03

### Added

- Range column types `int4range`, `int8range`, `numrange`, `tsrange`,
  `tstzrange`, and `daterange` in tags and migrations.
- Range values round-trip through pgx's native `pgtype.Range[T]` fields with no
  wrapper type.
- Exclusion constraints through a table-level
  `// exclude: <name> USING <method> (<elements>)` directive, for cases such as
  non-overlapping reservations.
- Exclusion constraint introspection from `pg_constraint`, diffing, and
  reconstruction from existing migration files.

## [1.20.0] - 2026-07-15

### Added

- Window functions through a fluent `builder.Window("expr")` type that drops
  into `Columns(...)` and scans its aliased result into a struct field.
- Named constructors `RowNumber`, `Rank`, `DenseRank`, `PercentRank`,
  `CumeDist`, `Ntile`, `Lag`, `Lead`, `FirstValue`, `LastValue`, and `NthValue`.
- Aggregate windows `SumOver`, `AvgOver`, `CountOver`, `MinOver`, and `MaxOver`.
- `.PartitionBy(...)`, `.OrderByAsc/Desc(...)`, `.Frame(...)`, and `.As(...)`
  modifiers. Window expressions carry no bound parameters, the same trust model
  as `Columns`.

## [1.19.0] - 2026-07-15

### Changed

- The transaction query builders delegate to one set of SQL builders and
  execution helpers behind a `queryExecutor` interface, satisfied by the pool
  directly and by a thin `pgx.Tx` adapter.
- `TxSelect`/`TxInsert`/`TxUpdate`/`TxDelete` previously carried full copies of
  the `ToSQL` bodies and had drifted, silently losing `LATERAL` join support
  until v1.17.4.
- `transaction.go` shrank from ~1120 to ~670 lines. The public API is unchanged,
  including `TxSelect(...).All()` with no `ctx`.
- New logo: a stacked-stone cairn with a Go-cyan crown pebble.
  `assets/images/logo.png` is the hero render, `assets/images/logo.svg` a flat
  vector mark for small sizes.

## [1.18.0] - 2026-07-15

### Changed

- The CLI loader (`pkg/loader`) and the runtime reflection parser (`pkg/schema`)
  funnel through one tag interpretation, differing only in how they extract
  Go-type facts. Both reimplemented tag parsing before, and had drifted.
- A parity test asserts the two paths produce identical metadata, so they cannot
  silently diverge again. Net ~340 fewer lines.

### Fixed

- The CLI loader dropped `index`, `enum(...)`, and `generated(...)` tags, so
  `pebble generate --models` omitted tag-defined indexes, enum type creation,
  and generated columns.
- With `--db` the differ could DROP a production index, because the code schema
  looked index-free. The loader now parses all of these, plus identity
  nullability and table-level `// index:` directives.
- `fk:table(column)` was not parsed by the reflection parser. Tag parsing
  checked for `(` before `:`, so `fk:users(id)` was split into the key
  `fk:users`.
- The AST loader infers PostgreSQL types for common Go built-in and
  standard-library types instead of defaulting untyped fields to `text`.

## [1.17.4] - 2026-07-15

### Fixed

- Query parameter placeholders collided across clauses. WHERE and HAVING each
  restarted numbering at `$1`, JOIN args were not offset, and CTE args were not
  accounted for, so any two arg-bearing clauses emitted duplicate `$1`s.
- Placeholders are numbered sequentially across JOIN, WHERE, and HAVING, and
  join and CTE fragments are renumbered to their position.
- Subquery condition helpers dropped their bound arguments. `InSubquery`,
  `ExistsSubquery`, `GtSubquery`, and the rest embedded the subquery SQL but
  discarded its args.
- Migrations with dollar-quoted bodies or semicolons in string literals were
  split incorrectly. A `$$`- and string-literal-aware splitter now backs both
  migration execution and offline reconstruction.
- The migration advisory lock ran on different pooled connections, so Unlock
  could land on another connection while the lock leaked on an idle one. It is
  now held on a dedicated connection for its lifetime.
- `TxSelect` join rendering emits the `LATERAL` keyword, matching the
  non-transaction builder.

## [1.17.3] - 2026-07-15

### Fixed

- `manyToMany` preloads silently returned empty slices. Junction-table primary
  keys were scanned into bare `interface{}`, yielding pgx's raw decoded types,
  which never compared equal to the struct-derived map keys.
- Junction values scan into the same Go type as the struct primary key fields,
  and the two queries are ordered so they never overlap on a single connection.
- Nested preloads panicked on value-element slices. `Preload("Posts.Comments")`
  where `Posts` is a `[]Post` crashed in reflect. Value elements are addressed
  before collection.

## [1.17.2] - 2026-07-15

### Fixed

- Multi-row INSERT could write values into the wrong columns. The column list
  came from the first row, but each later row re-ran the zero-value and identity
  skip logic independently, so its values misaligned against the column list.
- Rows 2..N emit values for exactly the first row's column list.
- `pebble migrate up --all` failed on any partially-migrated database. It passed
  already-applied migrations to the executor, which errors on them. Applied
  versions are filtered out first.
- Existing database enum types were invisible to introspection. `getEnumTypes`
  joined `pg_class`, which has no row for enum types, so the query always
  returned zero rows and every `generate --db` re-emitted `CREATE TYPE`.

## [1.17.1] - 2026-07-15

### Security

- Fixed SQL injection in `TSMatch`. It built `to_tsquery('<query>')` by string
  interpolation and emitted it as raw SQL, so a single quote in the search term
  broke out of the string literal. The query is now bound as a parameter.
- `ToTSQuery`, `JSONBPath`, and `JSONBPathText` escape single quotes in the
  literals they embed. Prefer `TSMatch` for untrusted input.

### Fixed

- PostgreSQL-specific operators now work in WHERE clauses. `WhereBuilder`
  rejected every operator outside the standard comparison set, so `@>`, `?`,
  `?|`, `?&`, `<@`, `&&`, `~`, and `@@` all failed at query time.
- Unknown operators are parameterized like comparisons and `Condition.Raw` is
  honored, which also makes every subquery helper work for the first time.
- `Preload` now works inside transactions, including nested dot-notation
  preloads. `TxSelectQuery.All()` and `First()` silently ignored them.
- Nil JSONB struct pointers insert as SQL NULL instead of an empty string, which
  PostgreSQL rejected.
- Reserved-word identifiers are quoted in builder SQL, generated migrations, and
  preload queries. Non-reserved names are emitted unchanged.
- `WithCTE(...).All(ctx)` executes the WITH clause. `CTESelect` inherited
  `All`/`First` by method promotion and silently dropped the CTEs.
- The `schema.StringArray` family failed to scan under pgx's default exec mode,
  where array results arrive in binary format. The builders route named Scanner
  slices on array columns through pgx's native array decoding.

### Changed

- Documented the intentional smart-default behavior: zero-valued fields on
  columns with a `default(...)` are omitted from INSERT so the database default
  applies. Use a pointer field to store an explicit zero.

## [1.16.6] - 2026-07-15

### Security

- Upgrade `github.com/jackc/pgx/v5` from v5.9.1 to v5.10.0. Includes the fix for
  SQL injection via placeholder confusion with dollar-quoted string literals
  (GHSA-j88v-2chj-qfwx), which matters because the migration executor runs
  statements with the simple protocol.
- v5.10.0 also hardens against malicious servers: bounded binary decoders,
  capped SCRAM iteration counts, TLS-encrypted cancel requests, and
  `require_auth`.

## [1.16.5] - 2026-03-27

### Changed

- Upgrade `github.com/jackc/pgx/v5` from `v5.7.6` to `v5.9.1`.

## [1.16.4] - 2026-03-05

### Fixed

- PostgreSQL array types (`text[]`, `integer[]`, `uuid[]`, etc.) stripped to
  their base type in CLI-generated migrations. Fields declared as `*[]string`
  with `po:"col,text[]"` now correctly emit `text[]` in `CREATE TABLE` SQL
  instead of `text`.
- Runtime schema parser (`GetSQLType`) now recognises explicit array-type tag
  options (`text[]`, `bigint[]`, etc.) rather than ignoring them and falling
  back to type inference.

## [1.16.3] - 2026-03-01

### Fixed

- `CREATE TABLE` statements in generated migrations now respect foreign key
  dependencies: referenced tables are always emitted before the tables that
  reference them. Previously, tables could appear in source-file order, causing
  FK constraint violations when applying migrations to PostgreSQL.

## [1.16.2] - 2026-03-01

### Fixed

- Foreign key constraints missing from all generated migrations. Three separate
  bugs:
  - AST loader (`pkg/loader`) never parsed `fk:table(column)` tag options —
    `ForeignKeys` was always empty.
  - Runtime parser (`pkg/schema`) read from `"db"` struct tag instead of `"po"`
    in `parseForeignKeys`.
  - Schema reconstructor (`pkg/migration`) did not parse
    `CONSTRAINT … FOREIGN KEY … REFERENCES` from existing migration SQL, causing
    all FKs to appear as newly added on every subsequent `pebble generate` run.
- Incremental `pebble generate` now emits only genuinely new tables/columns when
  FK constraints are present, with no spurious `ALTER TABLE ADD CONSTRAINT` for
  already-migrated tables.

### Changed

- `strings.IndexByte` + manual slice arithmetic replaced with `strings.Cut` in
  column definition parser.
- Untagged `switch` statements on character values converted to tagged `switch`
  in `typeTokenEnd` and index-definition tokenizer.

## [1.16.1] - 2026-02-23

### Fixed

- `pebble diff` build failure: dropped tables, columns, and indexes were passed
  as struct types to `%s` format verbs instead of their `.Name` fields.

## [1.16.0] - 2026-02-23

### Added

- Schema reconstruction from existing migration files as the offline baseline
  for `pebble generate`, eliminating the need for a separate schema snapshot
  JSON file. The CLI now replays all up-migration files in chronological order
  to determine the current schema state without a database connection.

### Fixed

- Down migrations now generate complete, executable SQL for all dropped items
  instead of placeholder comments:
  - Dropped tables → full `CREATE TABLE` statement reconstructed from original
    schema
  - Dropped columns → `ALTER TABLE ... ADD COLUMN` with original type and
    constraints
  - Dropped indexes → full `CREATE INDEX` with original definition
  - Dropped foreign keys → `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY`
  - Dropped constraints → `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE`
  - Dropped enum types → `CREATE TYPE ... AS ENUM` with original values
- Enum type modification down migrations now correctly describe the PostgreSQL
  limitation (values cannot be removed) rather than emitting a misleading TODO
  comment.

## [1.15.1] - 2026-02-16

### Fixed

- AST loader missing UNIQUE constraint metadata, causing spurious
  `DROP CONSTRAINT` in diff migrations.
- `pg_advisory_lock()` scan error from scanning `void` return into `*bool` in
  migration executor.

### Added

- Loader now generates `ConstraintMetadata` for `unique` columns, matching the
  reflection-based parser.
- Tests for UNIQUE constraint generation in AST loader.

## [1.15.0] - 2026-02-13

### Changed

- **BREAKING**: Minimum Go version bumped from 1.24 to 1.26.
- Modernized struct field iteration to use Go 1.26 `reflect.Type.Fields()`
  iterators in `parser.go` and `relationships.go`.
- Users benefit from Go 1.26 runtime improvements: Green Tea GC (10-40% lower GC
  overhead), faster `fmt.Errorf` allocations, improved slice stack allocation,
  and `io.ReadAll` performance.

## [1.14.6] - 2026-01-30

### Added

- Custom array types for PgBouncer/simple_protocol compatibility: `StringArray`,
  `Int32Array`, `Int64Array`, `Float64Array`, `BoolArray`.
- PostgreSQL text format array parsing for `{value1,value2,value3}` syntax.
- Full support for quoted elements, escaped characters, and NULL values in array
  parsing.
- Comprehensive test suite for array types (50+ test cases).

## [1.14.5] - 2026-01-25

### Fixed

- JSONB type incorrectly mapped to `json` instead of `jsonb` in CLI-generated
  migrations.
- TimestampTZ type incorrectly mapped to `timestamp` instead of `timestamptz` in
  CLI migrations.
- BigSerial type incorrectly mapped to `serial` instead of `bigserial` in CLI
  migrations.

### Added

- Comprehensive test suite for loader package SQL type detection.
- Missing PostgreSQL types `double precision` and `interval` to loader type
  list.

## [1.14.4] - 2026-01-13

### Changed

- Removed debug logging from `introspector.go` after verifying index recreation
  bug fix.

## [1.14.3] - 2026-01-13

### Fixed

- Index introspection failing when PostgreSQL returns schema-qualified table
  names.
- Empty `Columns` arrays in introspected indexes causing unnecessary index
  recreation.

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
- JSONB auto-marshaling returns string instead of `[]byte` for proper pgx
  encoding.

## [1.13.0] - 2026-01-09

### Added

- Native JSONB auto-marshaling for custom types without
  `driver.Valuer`/`sql.Scanner`.
- `IsJSONB` flag on `ColumnMetadata` for automatic serialization tracking.
- JSONB column detection from `jsonb`/`json` tag options or SQL type.
- `marshalJSONB()` helper for automatic JSON serialization on insert/update.
- `jsonbScanTarget` scanner for automatic JSON deserialization on select.
- Backward compatibility with types implementing `Value()`/`Scan()`.

## [1.12.0] - 2026-01-08

### Added

- Comprehensive PostgreSQL index support with column-level and table-level
  syntax.
- Column-level index tags: `index`, `index(name)`, `index(name,type)`,
  `index(name,type,desc)`.
- Table-level index comments for complex indexes with full PostgreSQL syntax.
- Expression indexes, partial indexes, and covering indexes support.
- Operator classes and collations for indexes.
- CONCURRENTLY flag for production-safe index creation.
- Full index introspection from existing databases.
- Index modification detection in schema differ.
- 148 comprehensive tests for index functionality.

## [1.11.0] - 2026-01-04

### Added

- Generic transaction functions: `TxSelect[T]`, `TxInsert[T]`, `TxUpdate[T]`,
  `TxDelete[T]`.
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

- Native JSONB struct scanning without wrapper types using pgx v5's
  `JSONBCodec`.
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
- Added pointer dereferencing before collecting key values in relationship
  loading.

## [1.8.2] - 2025-12-30

### Fixed

- Preload operations broken due to Go initialism handling in `toPascalCase()`.
- Rewrote `toPascalCase()` to recognize 38 common Go initialisms (ID, API, URL,
  etc.).

## [1.8.1] - 2025-12-30

### Fixed

- Preload failing with "table not registered" for custom table names.
- Added `TargetType` field to `RelationshipMetadata` for accurate registry
  lookup.

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

- PostgreSQL identity columns with `identity`, `identityAlways`,
  `identityByDefault` tags.
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

- `pebble generate metadata --scan ./path` for production-safe table name
  generation.
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

[unreleased]: https://github.com/marshallshelly/pebble-orm/compare/v1.27.0...HEAD
[1.27.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.26.0...v1.27.0
[1.26.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.25.1...v1.26.0
[1.25.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.25.0...v1.25.1
[1.25.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.24.1...v1.25.0
[1.24.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.24.0...v1.24.1
[1.24.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.23.0...v1.24.0
[1.23.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.22.0...v1.23.0
[1.22.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.21.0...v1.22.0
[1.21.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.20.0...v1.21.0
[1.20.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.19.0...v1.20.0
[1.19.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.18.0...v1.19.0
[1.18.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.17.4...v1.18.0
[1.17.4]: https://github.com/marshallshelly/pebble-orm/compare/v1.17.3...v1.17.4
[1.17.3]: https://github.com/marshallshelly/pebble-orm/compare/v1.17.2...v1.17.3
[1.17.2]: https://github.com/marshallshelly/pebble-orm/compare/v1.17.1...v1.17.2
[1.17.1]: https://github.com/marshallshelly/pebble-orm/compare/v1.17.0...v1.17.1
[1.17.0]: https://github.com/marshallshelly/pebble-orm/compare/v1.16.6...v1.17.0
[1.16.6]: https://github.com/marshallshelly/pebble-orm/compare/v1.16.5...v1.16.6
[1.16.5]: https://github.com/marshallshelly/pebble-orm/compare/v1.16.4...v1.16.5
[1.16.4]: https://github.com/marshallshelly/pebble-orm/compare/v1.16.3...v1.16.4
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
