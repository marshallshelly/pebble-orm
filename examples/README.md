# Examples - Production-Ready Structure

All examples now follow **production-ready project structure** with proper separation of concerns.

## 📁 Project Structure

Every example follows this structure:

```
example_name/
├── cmd/
│   └── example_name/
│       └── main.go           # Application entry point
├── internal/
│   ├── database/
│   │   └── db.go             # Database connection & config
│   └── models/
│       ├── models.go         # Domain models
│       └── registry.go       # Model registration
├── README.md                 # Example documentation
└── go.mod                    # Go module
```

## 🚀 Prerequisites

- **Go 1.24+**
- **PostgreSQL 14+**
- **Docker** (optional, for running PostgreSQL)

## 🐘 Running PostgreSQL with Docker

```bash
docker run --name pebble-postgres \
  -e POSTGRES_PASSWORD=password \
  -p 5432:5432 \
  -d postgres:alpine
```

## 📚 Examples

### 1. **Basic CRUD** (`basic/`)

**What it demonstrates:**

- ✅ Production project structure
- ✅ INSERT with RETURNING
- ✅ SELECT with WHERE, ORDER BY, LIMIT
- ✅ UPDATE with conditions
- ✅ DELETE operations
- ✅ COUNT queries
- ✅ Relationships with Preload
- ✅ Environment-based configuration

**Run it:**

```bash
cd basic
export DATABASE_URL="postgres://localhost:5432/pebble_basic?sslmode=disable"
go run cmd/basic/main.go
```

---

### 2. **Relationships** (`relationships/`)

**What it demonstrates:**

- ✅ **hasMany**: Author → Books (one-to-many)
- ✅ **belongsTo**: Book → Author (many-to-one)
- ✅ **hasOne**: User → Profile (one-to-one)
- ✅ **manyToMany**: User ↔ Roles (many-to-many)
- ✅ Eager loading with `Preload()` to prevent N+1 queries
- ✅ Multiple preloads in single query

**Models:**

```go
// hasMany
type Author struct {
    Books []Book `po:"-,hasMany,foreignKey(author_id)"`
}

// belongsTo
type Book struct {
    Author *Author `po:"-,belongsTo,foreignKey(author_id)"`
}

// hasOne
type User struct {
    Profile *Profile `po:"-,hasOne,foreignKey(user_id)"`
}

// manyToMany
type User struct {
    Roles []Role `po:"-,manyToMany,joinTable(user_roles)"`
}
```

**Run it:**

```bash
cd relationships
go run cmd/relationships/main.go
```

---

### 3. **Transactions** (`transactions/`)

**What it demonstrates:**

- ✅ Transaction blocks with automatic commit/rollback
- ✅ Savepoints for nested transactions
- ✅ Error handling and rollback
- ✅ Multiple operations in one transaction
- ✅ Transaction isolation

**Run it:**

```bash
cd transactions
go run cmd/transactions/main.go
```

---

### 4. **Migrations** (`migrations/`)

**What it demonstrates:**

- ✅ Schema introspection from database
- ✅ Schema definition from Go structs
- ✅ Diff generation (comparing schemas)
- ✅ Migration file generation (.up.sql / .down.sql)
- ✅ Migration execution with version tracking
- ✅ Rollback support

**Run it:**

```bash
cd migrations
go run cmd/migrations/main.go
```

---

### 5. **PostgreSQL Features** (`postgresql/`)

**What it demonstrates:**

- ✅ **JSONB**: Store and query JSON data
- ✅ **Arrays**: PostgreSQL array types
- ✅ **CTEs**: Common Table Expressions (WITH queries)
- ✅ **Subqueries**: Nested SELECT statements
- ✅ **Window Functions**: OVER, PARTITION BY
- ✅ **Full-Text Search**: tsvector, tsquery
- ✅ **Geometric Types**: point, line, polygon

**Run it:**

```bash
cd postgresql
go run cmd/postgresql/main.go
```

---

### 6. **Custom Table Names** (`custom_table_names/`)

**What it demonstrates:**

- ✅ Custom table names via `// table_name:` directive
- ✅ CLI metadata generation for production builds
- ✅ Default snake_case fallback
- ✅ Legacy database compatibility
- ✅ Table name mapping

**Example:**

```go
// table_name: custom_users_table
type User struct {
    ID int `po:"id,primaryKey,serial"`
}
// Creates table: "custom_users_table"

type Product struct {
    ID int `po:"id,primaryKey,serial"`
}
// Creates table: "product" (default snake_case)
```

**Production builds:**

```bash
# Generate metadata for Docker/production
pebble generate metadata --scan ./internal/models

# Generates table_names.gen.go with compile-time registrations
# Commit this file to version control!
```

**Run it:**

```bash
cd custom_table_names
go run cmd/custom_tables/main.go
```

---

### 8. **Generated Columns** (`generated_columns/`) ⭐ NEW

**What it demonstrates:**

- ✅ STORED generated columns (auto-computed values)
- ✅ String concatenation (full names)
- ✅ Unit conversions (cm to inches, kg to lbs)
- ✅ Complex calculations (net price with tax/discount)
- ✅ Querying generated columns
- ✅ Auto-update when source columns change

**Run it:**

```bash
cd generated_columns
go run cmd/generated/main.go
```

**Example Models:**

```go
type Person struct {
    FirstName string `po:"first_name"`
    LastName  string `po:"last_name"`
    // Auto-computed from first_name and last_name
    FullName  string `po:"full_name,generated:first_name || ' ' || last_name,stored"`
}

type Product struct {
    ListPrice float64 `po:"list_price"`
    Tax       float64 `po:"tax"`
    Discount  float64 `po:"discount"`
    // Auto-calculated net price
    NetPrice  float64 `po:"net_price,generated:(list_price + (list_price * tax / 100)) - (list_price * discount / 100),stored"`
}
```

---

### 7. **CASCADE DELETE** (`cascade_delete/`)

**What it demonstrates:**

- ✅ **CASCADE DELETE**: Automatically delete child records when parent is deleted
- ✅ **SET NULL**: Set foreign key to NULL on parent deletion
- ✅ **RESTRICT**: Prevent deletion if child records exist
- ✅ Database-level foreign key constraints
- ✅ Tag-based constraint configuration

**Models:**

```go
// CASCADE - Delete posts when user is deleted
type Post struct {
    AuthorID int `po:"author_id,integer,notNull"`
    // Foreign key with CASCADE defined in migration
}

// SET NULL - Keep comments but set author_id to NULL
type Comment struct {
    AuthorID *int `po:"author_id,integer"`
    // Foreign key with SET NULL defined in migration
}

// RESTRICT - Prevent category deletion if products exist
type Product struct {
    CategoryID int `po:"category_id,integer,notNull"`
    // Foreign key with RESTRICT defined in migration
}
```

**Run it:**

```bash
cd cascade_delete
go run cmd/cascade_delete/main.go
```

---

### 9. **GDPR-Compliant Multi-Tenancy** (`multi-tenancy/`) 🔒 PRODUCTION-READY

**What it demonstrates:**

- ✅ **GDPR Compliance**: Full implementation of GDPR requirements (Articles 5, 6, 7, 15, 17, 20, 32, 44-50)
- ✅ **REST API**: Production-ready API built with Go Fiber
- ✅ **Soft Delete**: Records marked as deleted, never removed (audit trail)
- ✅ **Audit Logging**: All actions logged with who, what, when, where
- ✅ **Consent Management**: Track user consent with timestamps
- ✅ **Data Portability**: Export user data in JSON format (Article 20)
- ✅ **Right to Erasure**: Soft delete, anonymization, deletion workflows (Article 17)
- ✅ **Tenant Isolation**: Automatic tenant_id filtering, prevents data leaks
- ✅ **Data Retention**: Configurable retention policies per tenant
- ✅ **Data Residency**: Track geographic data location

**GDPR Articles Implemented:**

| GDPR Article | Feature | Endpoint |
|--------------|---------|----------|
| **Article 5** | Accountability & Storage | Audit logs, retention policies |
| **Article 7** | Consent Management | `PUT /users/:id/consent` |
| **Article 15** | Right to Access | `GET /users/:id/audit-logs` |
| **Article 17** | Right to Erasure | `DELETE /users/:id/soft` |
| **Article 20** | Data Portability | `POST /users/:id/export` |
| **Article 32** | Security | Tenant isolation, audit logging |

**Models with GDPR Compliance:**

```go
// All models include GDPR metadata
type GDPRMetadata struct {
    CreatedAt   time.Time  `po:"created_at,timestamptz,default(NOW()),notNull"`
    CreatedBy   *string    `po:"created_by,uuid"`
    UpdatedAt   time.Time  `po:"updated_at,timestamptz,default(NOW()),notNull"`
    UpdatedBy   *string    `po:"updated_by,uuid"`
    DeletedAt   *time.Time `po:"deleted_at,timestamptz"`      // Soft delete
    DeletedBy   *string    `po:"deleted_by,uuid"`
    RetainUntil *time.Time `po:"retain_until,timestamptz"`    // Data retention
    Anonymized  bool       `po:"anonymized,boolean,default(false),notNull"`
}

// User with consent tracking
type User struct {
    ID       string `po:"id,primaryKey,uuid"`
    TenantID string `po:"tenant_id,uuid,notNull,index"`
    Name     string `po:"name,varchar(255),notNull"`
    Email    string `po:"email,varchar(320),notNull"`

    // Consent tracking (GDPR Article 7)
    MarketingConsent   bool       `po:"marketing_consent,boolean,default(false),notNull"`
    MarketingConsentAt *time.Time `po:"marketing_consent_at,timestamptz"`

    // Legal basis (GDPR Article 6)
    ProcessingBasis string `po:"processing_basis,varchar(50),default('consent'),notNull"`

    GDPRMetadata
}
```

**REST API Endpoints:**

```bash
# Create tenant with GDPR config
POST /api/v1/tenants
{
  "name": "Acme Corp",
  "data_region": "EU",
  "data_retention_days": 365
}

# Create user with consent
POST /api/v1/tenants/:tenantId/users
{
  "name": "Alice",
  "email": "alice@acme.com",
  "marketing_consent": true,
  "processing_basis": "consent"
}

# Update consent (GDPR Article 7)
PUT /api/v1/tenants/:tenantId/users/:userId/consent
{"consent_type": "marketing", "granted": false}

# Export user data (GDPR Article 20)
POST /api/v1/tenants/:tenantId/users/:userId/export

# Soft delete (GDPR Article 17)
DELETE /api/v1/tenants/:tenantId/users/:userId/soft

# Anonymize user
DELETE /api/v1/tenants/:tenantId/users/:userId/anonymize
```

**Automatic Tenant Filtering:**

```go
// TenantDB wrapper auto-filters all queries
tenantDB := database.NewTenantDB(qb, tenantID, userID)

// Excludes soft-deleted records automatically
users, err := database.SelectActive[models.User](tenantDB).All(ctx)
// SQL: SELECT * FROM users WHERE tenant_id = ? AND deleted_at IS NULL

// For admin/audit: includes deleted records
allUsers, err := database.SelectAll[models.User](tenantDB).All(ctx)
```

**Run the API Server:**

```bash
cd multi-tenancy
go run cmd/multi-tenancy/main.go

# Output:
🚀 GDPR-Compliant Multi-Tenant API Server starting on port 3000
📋 API Documentation: http://localhost:3000/api/v1

=== GDPR Features ===
✅ Soft Delete (Article 17)
✅ Audit Logging (Article 5)
✅ Data Portability (Article 20)
✅ Consent Management (Article 7)
✅ Right to Erasure (Article 17)
✅ Tenant Isolation
```

**Key Features:**

- 🔒 **GDPR-First Design**: Every feature built for compliance
- 🏢 **Multi-Tenant SaaS**: Production-ready architecture
- 📝 **Audit Trail**: Complete transparency, who accessed what
- 🔐 **Security**: Automatic tenant isolation, no data leaks
- ⚖️ **Legal Compliance**: Implements all major GDPR articles
- 🌍 **Data Residency**: Track where data is stored (EU/US/UK)
- 🎯 **Production Ready**: REST API, soft delete, retention policies

---

### 10. **PostgreSQL Indexes** (`indexes/`) ⚡ COMPREHENSIVE

**What it demonstrates:**

- ✅ **All PostgreSQL Index Types**: btree, gin, gist, brin, hash
- ✅ **Simple Column Indexes**: Auto-named and custom-named indexes
- ✅ **Expression Indexes**: Indexes on computed values (e.g., `lower(email)`)
- ✅ **Partial Indexes**: Conditional indexes with WHERE clauses
- ✅ **Covering Indexes**: INCLUDE columns for index-only scans
- ✅ **Operator Classes**: Pattern matching optimization (varchar_pattern_ops)
- ✅ **Collations**: Locale-specific sorting (en_US, C, etc.)
- ✅ **Column Ordering**: DESC/ASC with NULLS FIRST/LAST
- ✅ **Multicolumn Indexes**: Composite indexes with mixed ordering
- ✅ **CONCURRENTLY**: Production-safe index creation without blocking writes
- ✅ **GIN Indexes**: JSONB and array queries
- ✅ **BRIN Indexes**: Space-efficient time-series data
- ✅ **Hash Indexes**: Equality-only lookups

**12 Comprehensive Examples:**

1. **Product** - Simple column-level indexes
2. **User** - Expression and partial indexes
3. **Order** - Covering indexes with INCLUDE
4. **Event** - Multicolumn with mixed ordering
5. **SearchTerm** - Operator classes for pattern matching
6. **InternationalProduct** - Collations for locale-specific sorting
7. **AnalyticsEvent** - CONCURRENTLY for production
8. **Document** - Complex multi-feature indexes
9. **SensorReading** - BRIN for time-series data
10. **APIKey** - Hash indexes for equality-only
11. **Task** - Advanced NULLS ordering
12. **Article** - Composite with all features

**Index Types:**

```go
// Simple column index
Name string `po:"name,varchar(255),index"` // Auto-named: idx_table_name

// Custom name with type and ordering
CreatedAt time.Time `po:"created_at,timestamptz,index(idx_created,btree,desc)"`

// GIN index for JSONB
Metadata JSONB `po:"metadata,jsonb,index(idx_metadata,gin)"`

// Table-level complex indexes
// table_name: users
// index: idx_email_lower ON (lower(email))
// index: idx_active ON (email) WHERE deleted_at IS NULL
// index: idx_complex ON (col1, col2 DESC) INCLUDE (col3) WHERE active = true CONCURRENTLY
```

**Run it:**

```bash
cd indexes

# Generate migration
pebble generate --name create_index_tables --db "$DATABASE_URL"

# Apply migrations
pebble migrate up --all --db "$DATABASE_URL"

# Run examples
go run cmd/indexes/main.go
```

**Key Takeaways:**

- **btree**: Default, best for most queries (sorting, ranges, equality)
- **gin**: JSONB, arrays, full-text search
- **gist**: Geometric data, range types
- **brin**: Very large tables, time-series (100-1000x smaller than btree)
- **hash**: Fastest for equality-only lookups
- **Partial indexes**: Only index filtered data (smaller, faster)
- **Covering indexes**: INCLUDE enables index-only scans (huge performance boost)
- **CONCURRENTLY**: Build indexes without blocking writes in production

**Perfect for:**
- 🏗️ Understanding PostgreSQL index types and when to use them
- ⚡ Optimizing query performance
- 📊 Time-series and analytics workloads
- 🔍 Full-text and pattern matching searches
- 🏢 Production deployments with CONCURRENTLY

---

## 🎯 Common Patterns

### Environment Configuration

All examples support `DATABASE_URL` environment variable:

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/dbname?sslmode=disable"
```

If not set, each example uses a sensible default for its database name.

### Model Registration

Every example has centralized registration:

```go
// internal/models/registry.go
func RegisterAll() error {
    models := []interface{}{
        User{},
        Post{},
        // ... more models
    }

    for _, model := range models {
        registry.Register(model)
    }
    return nil
}
```

### Database Connection

Standard pattern across all examples:

```go
// internal/database/db.go
func Connect(ctx context.Context) (*runtime.DB, error) {
    connStr := os.Getenv("DATABASE_URL")
    models.RegisterAll()
    return runtime.ConnectWithURL(ctx, connStr)
}
```

### Main Application

Clean entry point in every example:

```go
// cmd/examplename/main.go
func main() {
    ctx := context.Background()

    db, err := database.Connect(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    qb := builder.New(db)

    // Business logic here...
}
```

---

## 🔒 Type-Safe Column Names

All examples demonstrate **builder.Col** for type-safe column name resolution:

### The Problem

Hardcoded strings are error-prone and violate DRY:

```go
// ❌ Two sources of truth - struct tag AND hardcoded string
type User struct {
    Email string `po:"email,varchar(255),unique"`
}

users, _ := builder.Select[User](qb).
    Where(builder.Eq("email", value)).  // ❌ Magic string - not type-safe!
    All(ctx)
```

### The Solution: `builder.Col`

**Single source of truth** through struct tags:

```go
// ✅ Column name defined ONLY in struct tag
type User struct {
    Email string `po:"email,unique"`  // ← One source of truth
}

// ✅ Type-safe column reference
users, _ := builder.Select[User](qb).
    Where(builder.Eq(builder.Col[User]("Email"), value)).
    All(ctx)
```

### Benefits

| Feature              | Benefit                                            |
| -------------------- | -------------------------------------------------- |
| **Single Source**    | Column names only in struct tags                   |
| **Type-Safe**        | `Col[User]("Email")` - wrong model = compile error |
| **Refactoring Safe** | IDE finds all field references                     |
| **Zero Overhead**    | Registry lookup at call time                       |
| **Autocomplete**     | IDE suggests valid field names                     |

### Example from `basic/`

```go
// Type-safe queries using builder.Col
users, err := builder.Select[User](qb).
    Where(builder.Gte(builder.Col[User]("Age"), 18)).
    OrderByDesc(builder.Col[User]("CreatedAt")).
    All(ctx)

// Update with type-safe column names
count, err := builder.Update[User](qb).
    Set(builder.Col[User]("Age"), 29).
    Where(builder.Eq(builder.Col[User]("Email"), "user@example.com")).
    Exec(ctx)
```

**All examples use `builder.Col` throughout!**

---

## 💡 Best Practices Demonstrated

1. ✅ **Separation of Concerns**: Models, DB, and application logic are separate
2. ✅ **Environment Variables**: Configuration from environment
3. ✅ **Error Handling**: Proper error checking and wrapping
4. ✅ **Resource Management**: `defer db.Close()` pattern
5. ✅ **Context Propagation**: Pass `context.Context` everywhere
6. ✅ **Centralized Registration**: Single place for model registration
7. ✅ **Clear Package Boundaries**: `internal/` for private code
8. ✅ **Modular Design**: Easy to test and maintain

---

## 🧪 Running All Examples

```bash
# Run all examples in sequence
for dir in basic relationships transactions migrations postgresql indexes custom_table_names cascade_delete generated_columns multi-tenancy; do
    echo "Running $dir example..."
    cd $dir
    go mod tidy
    go run cmd/*/*.go
    cd ..
done
```

---

## 📖 Learning Path

Recommended order for learning:

1. **basic/** - Start here for CRUD fundamentals
2. **custom_table_names/** - Learn schema customization
3. **relationships/** - Master data associations
4. **transactions/** - Understand atomicity
5. **postgresql/** - Explore advanced PostgreSQL features
6. **indexes/** - Master index types and query optimization
7. **migrations/** - Learn schema management
8. **cascade_delete/** - Master foreign key constraints and cascade actions
9. **generated_columns/** - Learn auto-computed column values
10. **multi-tenancy/** - Master multi-tenant architecture patterns

---

## 🎓 Key Takeaways

### Why This Structure?

**Before** (Old approach):

- ❌ Everything in one `main.go` file (150-200 lines)
- ❌ Hard to test individual components
- ❌ Difficult to scale as models grow
- ❌ Unclear separation of concerns

**After** (Production structure):

- ✅ Thin `main.go` (30-50 lines)
- ✅ Easy to unit test each package
- ✅ Scalable: add models without main.go bloat
- ✅ Clear responsibilities: models, database, application

### This Is Production-Ready

This structure is used in real-world Go applications at companies like:

- Google
- Uber
- HashiCorp
- And thousands of other production systems

---

## 🚀 Next Steps

After running the examples:

1. **Modify the code** - Change models, add fields
2. **Read the source** - Each example is well-commented
3. **Build something** - Use Pebble ORM in your own project
4. **Explore the CLI** - Try `pebble generate`, `pebble migrate`, etc.

---

## 📝 Documentation

- **Main README**: [`../README.md`](../README.md)
- **Implementation Guide**:
- **Migration Guide**: [`../PRODUCTION_STRUCTURE.md`](../PRODUCTION_STRUCTURE.md)

---

## ❓ Troubleshooting

### Connection Issues

```bash
# Check PostgreSQL is running
pg_isready

# Create missing databases
createdb pebble_basic
createdb pebble_relationships
createdb pebble_indexes_example
createdb pebble_multitenancy
# ...etc
```

### Module Issues

```bash
# In each example directory
go mod tidy
go mod download
```

### Import Issues

Make sure you're in the example directory:

```bash
cd examples/basic
go run cmd/basic/main.go  # ✅ Correct
```

Not from repository root:

```bash
go run examples/basic/cmd/basic/main.go  # ❌ Wrong
```

---

**All examples follow production-ready patterns you can use in real applications!** 🎉
