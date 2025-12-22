# Changelog

All notable changes to Pebble ORM will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [2.0.0] - 2025-12-22

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
