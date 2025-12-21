# Changelog

All notable changes to Pebble ORM will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
