# Changelog

All notable changes to Pebble ORM will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.8.4] - 2025-12-30

### Fixed

- **Critical**: Preload failing with pgx encode error `cannot find encode plan` for `ANY($1)` queries
  - Error: `unable to encode []interface{}{"uuid-string"} into text format for unknown type (OID 0)`
  - Root cause: Passing `[]interface{}` to pgx which requires typed slices for PostgreSQL array parameters
  - Added `convertToTypedSlice()` helper to convert `[]interface{}` to properly typed slices (`[]string`, `[]int`, etc.)
  - Applied fix to all 4 relationship load functions (belongsTo, hasOne, hasMany, manyToMany)
  - Impact: Preload now works correctly with pgx driver

## [1.8.3] - 2025-12-30

### Fixed

- **Critical**: Preload failing with pgx encode error when foreign keys are nullable pointers (`*string`, `*int`, etc.)
  - Error: `unable to encode []interface{}{(*string)(0x...)} into text format for unknown type (OID 0)`
  - Root cause: Relationship loading collected pointer values instead of dereferencing them
  - Fixed by adding pointer dereferencing before collecting foreign/primary key values in all 4 relationship load functions
  - Impact: Preload with nullable foreign keys now works correctly

## [1.8.2] - 2025-12-30

### Fixed

- **Critical**: Preload operations completely broken due to Go initialism handling

**🔴 P0: All Preload operations completely broken**

Fixed a critical bug in the `toPascalCase()` function that broke all relationship preloading by incorrectly converting database column names to Go struct field names.

#### Problem

The `toPascalCase()` helper function didn't follow Go naming conventions for initialisms (ID, URL, API, etc.), causing the preload system to look for non-existent struct fields.

**What was broken:**
```go
type Post struct {
    UserID int   `po:"user_id,integer,notNull"`  // Actual field: "UserID"
    User   *User `po:"-,belongsTo,foreignKey(user_id),references(id)"`
}

posts, _ := builder.Select[Post](db).
    Preload("User").  // ❌ FAILED - User field stayed nil
    All(ctx)

// posts[0].User == nil (not loaded!)
```

**Root Cause:**

The preload code converts `user_id` → `"UserId"` and then tries:
```go
fkField := item.FieldByName("UserId")  // ❌ NOT FOUND
```

But the actual Go struct field is `"UserID"` (not `"UserId"`), per Go convention that initialisms should be all-caps.

#### Incorrect Conversions

```go
// Before fix (WRONG):
toPascalCase("user_id") → "UserId"      // Should be "UserID"
toPascalCase("id") → "Id"                // Should be "ID"
toPascalCase("api_key") → "ApiKey"      // Should be "APIKey"
toPascalCase("http_url") → "HttpUrl"    // Should be "HTTPURL"
```

#### Solution

Rewrote `toPascalCase()` to recognize 38 common Go initialisms (ID, API, URL, HTTP, JSON, SQL, UUID, etc.) and convert them to all-caps:

```go
// After fix (CORRECT):
toPascalCase("user_id") → "UserID"      ✅
toPascalCase("id") → "ID"                ✅
toPascalCase("api_key") → "APIKey"      ✅
toPascalCase("http_url") → "HTTPURL"    ✅
toPascalCase("created_by_user_id") → "CreatedByUserID" ✅
```

#### Impact

**Before fix:**
- `belongsTo`: ❌ Completely broken
- `hasOne`: ❌ Completely broken
- `hasMany`: ❌ Completely broken
- `manyToMany`: ❌ Completely broken

**After fix:**
- `belongsTo`: ✅ Working
- `hasOne`: ✅ Working
- `hasMany`: ✅ Working
- `manyToMany`: ✅ Working

#### Files Changed

- `pkg/builder/relationships.go`: Rewrote `toPascalCase()` with initialism support
- `pkg/builder/relationships_test.go`: Updated tests + added 27 comprehensive test cases
- All relationship preload operations now work correctly

#### Supported Initialisms

ACL, API, ASCII, CPU, CSS, DNS, EOF, GUID, HTML, HTTP, HTTPS, ID, IP, JSON, LHS, QPS, RAM, RHS, RPC, SLA, SMTP, SQL, SSH, TCP, TLS, TTL, UDP, UI, UID, UUID, URI, URL, UTF8, VM, XML, XMPP, XSRF, XSS

Based on: https://github.com/golang/lint/blob/master/lint.go

#### Testing

- Added `TestToPascalCase` with 27 comprehensive test cases
- Updated existing helper function tests
- All 200+ tests pass ✅
- Verified field lookup works correctly with reflection

## [1.8.1] - 2025-12-30

### Fixed - Critical Preload Bug with Custom Table Names

**🔴 P0: Preload failing with "table not registered" error**

Fixed a critical bug where `Preload()` would fail when relationship target models use custom table names that differ from their Go struct names.

#### Problem

When using relationships with models that have custom table names (e.g., `Asset` struct with `table_name: assets`), Preload would fail with:

```
ERROR: target table asset not registered
```

**Example that failed:**

```go
// Model definition
// table_name: assets
type Asset struct {
    ID string `po:"id,primaryKey,uuid"`
}

type Team struct {
    ProfileImage *Asset `po:"-,belongsTo,foreignKey(profile_image_id),references(id)"`
}

// This would fail:
team, _ := builder.Select[Team](db).
    Preload("ProfileImage").  // ERROR: target table "asset" not registered
    First(ctx)
```

**Root Cause:**

The relationship parser assumed table names matched Go struct names in snake_case (`Asset` → `asset`), but ignored custom table names defined in model comments. When `Preload()` tried to look up `"asset"` in the registry, it failed because the model was registered as `"assets"`.

#### Solution

Added `TargetType reflect.Type` field to `RelationshipMetadata`:

- Stores the actual Go type during relationship parsing
- Preload now uses `registry.Get(rel.TargetType)` for accurate table lookup
- Falls back to `registry.GetByName(rel.TargetTable)` for backward compatibility

**Changes:**

- `pkg/schema/metadata.go`: Added `TargetType` field to `RelationshipMetadata`
- `pkg/schema/relationships.go`: Store target type in `parseRelationship()`
- `pkg/builder/relationships.go`: Updated all load functions to use `TargetType` for registry lookup

**After fix:**

```go
team, _ := builder.Select[Team](db).
    Preload("ProfileImage").  // ✅ Works! Uses TargetType for accurate lookup
    First(ctx)
```

#### Impact

- **High Impact**: Preload is a core feature; this affected any multi-tenant or custom-table-name scenarios
- **Backward Compatible**: Falls back to old behavior if `TargetType` is nil
- **Fixed in**: All relationship types (belongsTo, hasOne, hasMany, manyToMany)

## [1.8.0] - 2025-12-29

### Added - Smart Default Value Detection

**Major Developer Experience Improvement**: Automatic zero-value omission for fields with database defaults.

#### Problem Solved

Previously, using non-pointer types with database-generated values caused errors:

```go
// ❌ Before - Required pointers or caused errors
type User struct {
    ID    string `po:"id,uuid,default(gen_random_uuid())"`
    Email string `po:"email,text,notNull"`
}

user := User{Email: "test@test.com"}
// ID is "" (empty string)
// INSERT INTO users (id, email) VALUES ('', 'test@test.com')
// ERROR: invalid input syntax for type uuid: ""
```

**Workaround was verbose**: Required pointers for all auto-generated fields:

```go
type User struct {
    ID    *string `po:"id,uuid,default(gen_random_uuid())"`  // Pointer!
    Email string  `po:"email,text,notNull"`
}
```

#### Smart Solution

Pebble ORM now **automatically omits zero-valued fields** that have database defaults:

```go
// ✅ After - Natural, intuitive syntax
type User struct {
    ID        string    `po:"id,uuid,default(gen_random_uuid())"`
    Email     string    `po:"email,text,notNull"`
    CreatedAt time.Time `po:"created_at,timestamptz,default(NOW())"`
}

user := User{Email: "test@test.com"}
// ID is ""          → has default() → OMITTED ✅
// CreatedAt is zero → has default() → OMITTED ✅
// INSERT INTO users (email) VALUES ('test@test.com')
// RETURNING id, created_at; -- Database generates these
```

#### How It Works

The INSERT builder intelligently omits fields when **ALL** of these are true:

1. Field has a `default()` tag OR is an `identity` column
2. Field value is Go's zero value (`""`, `0`, `time.Time{}`, `nil`, etc.)

#### Explicit Values Still Work

Non-zero values are always included:

```go
user := User{
    ID:    "custom-uuid-1234",  // Explicit value
    Email: "test@test.com",
}
// INSERT INTO users (id, email) VALUES ('custom-uuid-1234', 'test@test.com')
```

#### Edge Cases Handled

**Pointers still work** (backward compatible):

```go
type User struct {
    ID *string `po:"id,uuid,default(gen_random_uuid())"`
}
user := User{ID: nil}  // nil is zero → omitted ✅
```

**Fields without defaults are always included**:

```go
type User struct {
    Email string `po:"email,text,notNull"`  // No default
    Age   int    `po:"age,integer"`         // No default
}
user := User{Email: "", Age: 0}
// Both included even though zero (no defaults)
// INSERT INTO users (email, age) VALUES ('', 0)
```

**Identity columns also omitted when zero**:

```go
type Product struct {
    ID   int64  `po:"id,bigint,identity"`
    Name string `po:"name,text,notNull"`
}
product := Product{Name: "Widget"}  // ID is 0
// INSERT INTO products (name) VALUES ('Widget')
// RETURNING id; -- Database generates
```

### Benefits

- ✅ **Intuitive**: Works how developers expect
- ✅ **Less boilerplate**: No pointers for generated fields
- ✅ **Fewer errors**: Prevents "invalid UUID" mistakes
- ✅ **Better DX**: New users don't hit this immediately
- ✅ **Backward compatible**: Pointers still work
- ✅ **Clearer models**: Field nullability matches database reality

### Technical Details

- Updated `pkg/builder/scanner.go`: Added smart default detection to `structToValues()`
- Uses Go's `reflect.Value.IsZero()` for accurate zero detection
- Checks `col.Default != nil` and `col.Identity != nil`
- Added comprehensive test suite: `pkg/builder/smart_defaults_test.go`

### Migration Guide

**No breaking changes!** Both patterns work:

```go
// Old pattern (still works)
type User struct {
    ID *string `po:"id,uuid,default(gen_random_uuid())"`
}

// New pattern (cleaner)
type User struct {
    ID string `po:"id,uuid,default(gen_random_uuid())"`
}
```

Gradually migrate by removing pointers as you update models.

### Comparison with Other ORMs

Pebble ORM now matches the intuitive behavior of:

- **GORM**: Auto-omits zero primary keys
- **Ent**: Handles defaults intelligently
- **SQLBoiler**: Smart null handling

## [1.7.2] - 2025-12-29

### Fixed - Critical Production Issues

- **🔴 P0: Constraint-backed index detection**  
  Fixed migration planner generating `DROP INDEX` statements for indexes that back UNIQUE/PRIMARY KEY constraints, which caused production failures:

  ```
  ERROR: cannot drop index users_email_key because constraint users_email_key
  on table users requires it (SQLSTATE 2BP01)
  ```

  **Root Cause**: Introspector didn't distinguish between standalone indexes and constraint-backed indexes.

  **Fix**: Added `LEFT JOIN pg_constraint` to exclude constraint-backed indexes from standalone index operations.

  **Impact**: ✅ Migrations no longer attempt to drop constraint-backed indexes

- **🔴 P0: Missing USING clauses for type conversions**  
  Fixed type conversions that require explicit casting logic, which blocked schema evolution:

  ```
  ERROR: column "languages_spoken" cannot be cast automatically to type text[]
  (SQLSTATE 42804)
  ```

  **Root Cause**: Migration planner generated simple `ALTER TABLE ... TYPE` without USING clauses for incompatible conversions.

  **Fix**: Added automatic USING clause generation for common conversions:

  - `text/varchar` → `text[]`: Null-safe array wrapping
  - `text/varchar` → `jsonb`: Null-safe JSON parsing
  - `text/varchar` → `json`: Null-safe JSON parsing
  - `text/varchar` → `integer`: Regex-validated conversion

  For unsupported conversions, generates commented-out statements with manual intervention instructions.

  **Impact**: ✅ Type conversions now work automatically for common cases, with safety guidance for complex cases

### Added

- **Production Safety Guide**: New `PRODUCTION_SAFETY.md` documenting best practices for using migrations in production
- **Type Conversion Detection**: `requiresUsingClause()` function to detect incompatible PostgreSQL type conversions
- **Smart USING Generation**: `generateUsingClause()` provides safe default conversions for common type changes

### Technical Details

- Updated `introspector.go`: Added constraint detection to `getIndexes()` query
- Updated `planner.go`: Added type conversion logic with USING clause generation
- Added comprehensive tests: `constraint_index_test.go` and `type_conversion_test.go`
- Updated `.gitignore`: Whitelisted PRODUCTION_SAFETY.md

### Migration Example

**Before v1.7.2 (Fails):**

```sql
ALTER TABLE teams ALTER COLUMN languages_spoken TYPE text[];
-- ERROR: cannot be cast automatically
```

**After v1.7.2 (Works):**

```sql
ALTER TABLE teams ALTER COLUMN languages_spoken TYPE text[]
USING CASE
  WHEN languages_spoken IS NULL THEN NULL
  WHEN languages_spoken = '' THEN ARRAY[]::text[]
  ELSE ARRAY[languages_spoken]::text[]
END;
```

### Breaking Changes

None. All changes are backward-compatible and improve existing functionality.

### Upgrade Path

1. Update to v1.7.2: `go get github.com/marshallshelly/pebble-orm@v1.7.2`
2. Regenerate any pending migrations
3. Review auto-generated USING clauses before applying

### Known Limitations

- Auto-migration still runs on first request (Issue #3). See PRODUCTION_SAFETY.md for workarounds.
- Manual intervention required for complex type conversions (e.g., `integer` → `jsonb`)

## [1.7.1] - 2025-12-28

### Fixed

- **Critical: Migration Generation fixes**
  - **Phantom ALTER commands**: Fixed type normalization bug where `timestamp` vs `timestamp without time zone` and `NOW()` vs `now()` caused unnecessary migration statements
  - **Duplicate UNIQUE indexes**: Removed redundant `CREATE UNIQUE INDEX` statements for `UNIQUE` columns (PostgreSQL creates them implicitly)
  - **Identity Columns in CLI**: Fixed `pebble generate` command not parsing `identity` tags when running from source (loader bug)
  - **Identity SQL Generation**: Fixed syntax error in `CREATE TABLE` for identity columns with other constraints

### Improved

- **Cleaner SQL Generation**: `CREATE TABLE` now uses inline `PRIMARY KEY` for single-column PKs instead of verbose `CONSTRAINT` syntax
- **Modern Go API**: Replaced deprecated `reflect.PtrTo` with `reflect.PointerTo` (Go 1.18+)
- **Code Cleanup**: Removed dead code (`extractPackagePath`)

### Technical Details

- Updated `loader` to correctly parse `identity`, `identityAlways`, and `identityByDefault` tags from AST
- Enhanced `differ` to normalize PostgreSQL types and default values for accurate schema comparison
- Updated `planner` to generate concise SQL for primary keys
- Fixed critical regression where identity columns were ignored during CLI generation

### Added

- **PostgreSQL Identity Columns Support** - Added support for SQL standard` GENERATED AS IDENTITY` columns
  - **Modern Alternative to SERIAL**: Identity columns are the SQL:2003 standard for auto-incrementing IDs
  - **Two Generation Types**:
    - `identity` or `identityAlways` → `GENERATED ALWAYS AS IDENTITY` (strict, prevents manual override)
    - `identityByDefault` → `GENERATED BY DEFAULT AS IDENTITY` (flexible, allows manual values)

### Tag Syntax

```go
type User struct {
    // GENERATED ALWAYS AS IDENTITY (recommended)
    ID int64 `po:"id,primaryKey,bigint,identity"`

    // GENERATED BY DEFAULT AS IDENTITY
    OrderID int `po:"order_id,integer,identityByDefault"`

    // Legacy SERIAL (still works)
    LegacyID int `po:"legacy_id,serial"`
}
```

### Generated SQL

```sql
CREATE TABLE users (
    id bigint GENERATED ALWAYS AS IDENTITY,
    order_id integer GENERATED BY DEFAULT AS IDENTITY,
    legacy_id serial NOT NULL,
    CONSTRAINT users_pkey PRIMARY KEY (id)
);
```

### Implementation Details

**Schema Metadata:**

- Added `Identity *IdentityColumn` field to `ColumnMetadata`
- New types: `IdentityColumn`, `IdentityGeneration` (ALWAYS | BY DEFAULT)

**Parser:**

- Recognizes `identity`, `identityAlways`, `identityByDefault` tags
- Identity columns automatically NOT NULL (per PostgreSQL spec)

**Migration Planner:**

- Generates `GENERATED {ALWAYS|BY DEFAULT} AS IDENTITY` syntax
- Proper handling in CREATE TABLE statements

**Testing:**

- Comprehensive test coverage in `pkg/schema/identity_test.go`
- Migration generation tests in `pkg/migration/identity_test.go`
- Example documentation in `examples/identity-columns/`

### Benefits

- ✅ **SQL Standard** - Portable, future-proof
- ✅ **Better Safety** - `ALWAYS` prevents accidental manual IDs
- ✅ **Cleaner** - No implicit sequence creation
- ✅ **Modern** - PostgreSQL documentation recommends over SERIAL
- ✅ **Backward Compatible** - Doesn't break existing SERIAL columns

### References

- [PostgreSQL Documentation: Identity Columns](https://www.postgresql.org/docs/current/ddl-identity-columns.html)
- Example: `examples/identity-columns/README.md`

## [1.6.1] - 2025-12-27

### Fixed

- **Critical: Serial Column DROP DEFAULT Bug** - Fixed auto-migration incorrectly generating `DROP DEFAULT` for serial columns
  - **Problem**: When comparing code schema (serial) vs database schema (integer with nextval), differ saw these as different defaults
  - **Impact**: Auto-increment functionality broken after migration, causing `NOT NULL` constraint violations on INSERT
  - **Root Cause**: Differ didn't recognize that `serial` in code === `DEFAULT nextval('table_id_seq'::regclass)` in database
  - **Solution**: Added special handling to detect and preserve sequence-based defaults for serial/autoincrement columns
  - **Affects**: Runtime auto-migration (`initSchemaWithMigrations`), not file-based migrations

### Technical Details

The differ now recognizes these as equivalent (no migration generated):

```go
// Code schema
type Model struct {
    ID int `po:"id,primaryKey,serial"`
}

// Database schema
CREATE TABLE model (
    id INTEGER NOT NULL DEFAULT nextval('model_id_seq'::regclass)
);
```

**New Helper Methods:**

- `isSameDefaultWithSerial()` - Compares defaults with serial column awareness
- `isAutoIncrementColumn()` - Detects serial/bigserial/smallserial types
- `isSequenceDefault()` - Identifies PostgreSQL sequence defaults (nextval patterns)

**Test Coverage:**

- Serial, bigserial, smallserial variations
- Different nextval() formats (with/without regclass, quoted names, uppercase)
- Ensures legitimate default changes still detected
- Comprehensive test suite: `pkg/migration/serial_default_test.go`

## [1.6.0] - 2025-12-27

### Added

- **CLI Migration Generation from Source Files**: Generate migrations directly from Go source files without requiring a database connection

  - `pebble generate --name migration_name --models ./path/to/models`
  - No `--db` flag required for initial migrations
  - Scans `.go` files and builds schema from struct tags
  - Supports both single files and directories
  - Respects custom table names from `// table_name:` comments
  - Generates complete CREATE TABLE statements with all constraints

- **AST-Based Schema Building**: New loader package that parses Go source files

  - Direct `TableMetadata` construction from AST
  - Extracts columns, types, constraints from struct tags
  - Handles primary keys, unique indexes, defaults
  - No reflection or runtime type information needed

- **Registry.RegisterMetadata()**: New method for direct `TableMetadata` registration
  - Allows CLI tools to register schemas without Go types
  - Bypasses the need for actual struct instances
  - Enables migration generation from source code alone

### Changed

- **`--db` Flag Now Optional**: Database connection only required when comparing with existing schema

  - Without `--db`: Generates initial migration (treats DB as empty)
  - With `--db`: Generates diff-based migration (compares code vs database)
  - Makes initial project setup easier

- **Migration Workflow Simplified**:

  ```bash
  # Step 1: Define models in Go
  # Step 2: Generate migration (no database needed!)
  pebble generate --name initial_schema --models ./internal/models

  # Step 3: Apply migration
  pebble migrate up --db "postgres://..."
  ```

### Benefits

- ✅ **No Database Required**: Generate migrations before database even exists
- ✅ **Source of Truth**: Go structs define schema, CLI generates SQL
- ✅ **Faster Development**: No need to manually write CREATE TABLE statements
- ✅ **Custom Table Names**: Automatically extracts from comments
- ✅ **Type Safe**: Generates SQL from strongly-typed Go definitions

## [1.5.3] - 2025-12-23

### Fixed

- **Serial Type in Migrations**: Fixed critical bug where migration planner generated invalid `ALTER COLUMN TYPE serial` SQL
  - **Problem**: `serial` is a PostgreSQL pseudotype that only works in `CREATE TABLE`, not `ALTER TABLE`
  - **Root Cause**: Differ compared `serial` (from code) vs `integer` (from database introspection) as different types
  - **Impact**: Migrations failed with `ERROR: type "serial" does not exist (SQLSTATE 42704)`
  - **Solution**: Map `serial` types to their underlying base types in type normalization
    - `serial` / `serial4` → `integer`
    - `bigserial` / `serial8` → `bigint`
    - `smallserial` / `serial2` → `smallint`
  - Now correctly recognizes that `serial` in code equals `integer` in database (no type change)
  - Never generates `ALTER COLUMN TYPE serial` (uses `integer` instead)

### Technical Details

PostgreSQL serial types are syntactic sugar:

```sql
-- What you write:
CREATE TABLE users (id serial PRIMARY KEY);

-- What PostgreSQL creates:
CREATE SEQUENCE users_id_seq;
CREATE TABLE users (
    id integer NOT NULL DEFAULT nextval('users_id_seq'),
    PRIMARY KEY (id)
);
```

Since `serial` is not a real type, `ALTER TABLE ... TYPE serial` fails. The differ now treats:

- Code: `serial` ≡ Database: `integer` (no diff)
- This prevents invalid ALTER statements

### Testing

Added comprehensive test suite (`pkg/migration/serial_test.go`):

- `TestSerialTypesMapToInteger` - Validates type equivalence
- `TestSerialDoesNotTriggerTypeChange` - Ensures no false type diffs
- `TestBigSerialMapping` - Tests bigserial ↔ bigint
- All existing migration tests pass

## [1.5.2] - 2025-12-23

### Added

- **CLI Metadata Generation**: Production-safe solution for custom table names via code generation
  ```bash
  pebble generate metadata --scan ./internal/models
  ```
  - Scans source files for `// table_name:` comment directives
  - Generates `table_names.gen.go` with compile-time registrations
  - Generated code is committed to version control
  - Works in production Docker builds (no runtime source file dependency)

### Fixed

- **Production Build Support**: v1.5.1's comment parsing requires source files at runtime
  - Problem: Multi-stage Docker builds only copy compiled binary, not `.go` files
  - Impact: Custom table names fail in production, fall back to wrong snake_case names
  - Solution: CLI generates code that registers table names at compile-time

### Changed

- **Table Name Resolution Priority**:
  1. Global registry (populated by generated code from `pebble generate metadata`)
  2. Comment directives (development only, when source files exist)
  3. snake_case fallback (default)

### Workflow

```bash
# 1. Write models with comments (as before)
# // table_name: tenants
# type Tenant struct { ... }

# 2. Generate metadata before building
pebble generate metadata --scan ./api/models

# 3. Commit generated file
git add ./api/models/table_names.gen.go

# 4. Build Docker (generated code is included)
docker build -t app:latest .

# ✅ Custom table names work in production!
```

### Why This Approach

- ✅ **Zero boilerplate** in model files
- ✅ **Comments still work** - no API changes
- ✅ **CLI-driven** - fits existing workflow
- ✅ **Production-safe** - works in Docker
- ✅ **One command** - `pebble generate metadata`

## [1.5.1] - 2025-12-23

### Fixed

- **table_name Comment Directive**: Fixed critical bug where `// table_name: custom_name` comments were not being parsed
  - Issue: Models registered with custom table names were falling back to default snake_case conversion
  - Root Cause: `findSourceFile()` function couldn't locate source files for main package or models outside GOPATH
  - Solution: Enhanced source file search to prioritize current working directory
  - Impact: Custom table names now work correctly for migrations and queries
  - Affected: Model registration, schema parsing, migration generation

### Changes

- Updated `pkg/schema/parser.go`:
  - `findSourceFile()` now searches current working directory first
  - Properly handles `main` package models
  - Works with models defined anywhere in the filesystem
- Updated tests to expect correct custom table names instead of fallback values

## [1.5.0] - 2025-12-23

### Added

- **PostgreSQL Generated Columns**: Full support for `GENERATED ALWAYS AS` columns
  - `STORED` generated columns (computed on INSERT/UPDATE)
  - `VIRTUAL` type reserved for future PostgreSQL support
  - Tag syntax: `po:"column_name,generated:EXPRESSION,stored"`
  - Automatic SQL generation: `GENERATED ALWAYS AS (expression) STORED`
  - Comprehensive test coverage

### Features

- **Automatic Computation**: Database computes values from other columns
- **Type-Safe**: Defined in Go structs with struct tags
- **Migration Support**: Generates correct DDL automatically
- **Read-Only**: Generated columns cannot be manually set
- **Indexable**: Can create indexes on generated columns

### Examples

```go
type Person struct {
    FirstName string `po:"first_name"`
    LastName  string `po:"last_name"`
    FullName  string `po:"full_name,generated:first_name || ' ' || last_name,stored"`
}
```

### Documentation

- Added comprehensive generated columns example
- Updated parser to support colon format (`key:value`)
- Added tests for schema parsing and SQL generation

## [1.4.0] - 2025-12-23

### Added

- **Safe Auto-Migrations**: Migrations are now idempotent by default
  - `CREATE TABLE IF NOT EXISTS` instead of `CREATE TABLE`
  - `CREATE INDEX IF NOT EXISTS` for all indexes
  - Safe to run migrations multiple times without errors
  - Configurable via `PlannerOptions.IfNotExists` (default: `true`)
  - New `NewPlannerWithOptions()` for custom migration behavior

### Benefits

- ✅ Applications can restart without migration errors
- ✅ Deployments are more robust
- ✅ No manual error handling needed
- ✅ Aligns with database migration best practices

### Changed

- **Migration Planner**: Now uses `PlannerOptions` struct for configuration
- **Default Behavior**: All migrations include `IF NOT EXISTS` (safe by default)

### Documentation

- Added comprehensive test coverage for `PlannerOptions`
- Updated all migration tests to expect `IF NOT EXISTS`

## [1.3.1] - 2025-12-22

### Documentation

- **Example Updates**: Updated `transactions` and `relationships` examples to use `builder.Col`
  - Replaced hardcoded string literals with type-safe column references
  - Ensures consistency with v1.3.0 features
  - Fixed linting warnings in example code

## [1.3.0] - 2025-12-22

### ⚠️ BREAKING CHANGES

- **Tag Naming**: Foreign key constraint tags now use camelCase for consistency
  - Changed: `ondelete:` → `onDelete:`
  - Changed: `onupdate:` → `onUpdate:`
  - **Migration**: Run `sed -i 's/ondelete:/onDelete:/g' **/*.go` and `sed -i 's/onupdate:/onUpdate:/g' **/*.go`
  - Reason: Matches existing camelCase convention (`notNull`, `primaryKey`, `autoIncrement`, etc.)

### Added

- **Type-Safe Column Names**: `builder.Col[T](fieldName)` helper (from v1.2.1)
  - Single source of truth through struct tag metadata
  - Compile-time type safety with Go generics
  - IDE autocomplete support
- **pkg.go.dev Badge**: Official documentation badge

### Changed

- **Tag Consistency**: All struct tag options now use camelCase
- **Integration Tests**: Use ORM's migration system (validates ORM features)
- **Builder API**: Consistent `builder.Select[T](qb)` pattern throughout

### Fixed

- Tag naming inconsistency across foreign key constraints
- GoReleaser deprecation warnings
- Integration test compilation

## [1.2.1] - 2025-12-22

### Added

- **Type-Safe Column Names**: New `builder.Col[T](fieldName)` helper for single source of truth
  - Extracts database column names from struct tag metadata
  - Eliminates hardcoded string literals in queries
  - Provides compile-time type safety with Go generics
  - Zero runtime overhead via registry lookup
  - IDE-friendly with autocomplete support
- **pkg.go.dev Badge**: Added official Go package documentation badge to README

### Changed

- **Integration Tests**: Now use Pebble ORM's migration system instead of raw SQL
  - Tests schema introspection, diffing, and SQL generation
  - Validates ORM's own migration capabilities
  - Demonstrates real-world migration usage
- **Builder API**: Updated all examples to use `builder.Select[T](qb)` pattern
  - Consistent API across all operations
  - Removed legacy `db.Insert(T{})` patterns
- **Documentation**: Comprehensive updates across all files
  - README.md: Quick start with `builder.Col`
  - CLAUDE.md: All query examples use type-safe column names
  - examples/README.md: New "Type-Safe Column Names" section
  - examples/basic: Live demonstration of `builder.Col`

### Fixed

- GoReleaser configuration deprecation warnings
- Removed non-existent Homebrew tap from release config
- Integration test compilation with new builder API
- Removed unused `toTyped` helper function

### Documentation

- Added MIT LICENSE file
- Enhanced examples README with builder.Col best practices
- Updated all code examples to demonstrate single source of truth
- Added benefits table and problem/solution comparison

## [1.1.0] - 2025-01-XX

project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-12-22

### Added

#### Core Features

- Type-safe query builder using Go 1.21+ generics
- Zero-overhead performance with native pgx integration
- Struct-tag based schema definitions
- Automatic migration generation and management
- Full relationship support (hasMany, hasOne, belongsTo, manyToMany)
- CASCADE DELETE via foreign key constraint tags
- Transaction support with proper error handling

#### PostgreSQL Features

- JSONB support for flexible JSON data
- Array types for multi-value columns
- UUID primary key support
- Geometric types (point, polygon, circle, etc.)
- Full-text search capabilities

#### Developer Experience

- Interactive CLI with TUI for migrations and introspection
- 7 comprehensive examples with production-ready structure
  - Basic CRUD operations
  - Custom table names
  - Relationships with eager loading
  - Transaction handling
  - Migration management
  - PostgreSQL-specific features
  - CASCADE DELETE examples
- Complete API documentation
- Integration tests with testcontainers

#### Architecture

- `pkg/builder`: Type-safe query builder
- `pkg/schema`: Schema parsing and type mapping
- `pkg/registry`: Model registration and caching
- `pkg/migration`: Migration system and introspection
- `pkg/runtime`: Database connection management
- `cmd/pebble`: Interactive CLI tool

### Documentation

- Comprehensive README with badges and examples
- Professional logo with transparent background
- Individual README files for all 7 examples
- Migration guide and best practices
- CI/CD workflows for testing and releases

### Infrastructure

- GitHub Actions CI workflow
- golangci-lint integration
- GoReleaser configuration for multi-platform releases
- Homebrew tap support

[1.0.0]: https://github.com/marshallshelly/pebble-orm/releases/tag/v1.0.0
