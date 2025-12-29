# Multi-Tenancy Example

This example demonstrates how to implement multi-tenant architectures with Pebble ORM using two different patterns:

1. **Shared Database with tenant_id** - Single database with automatic tenant filtering
2. **Database-per-Tenant** - Separate database for each tenant

## Project Structure

```
multi-tenancy/
├── cmd/
│   └── multi-tenancy/
│       └── main.go           # Demonstrates both patterns
├── internal/
│   ├── database/
│   │   └── db.go             # TenantDB wrapper & TenantManager
│   └── models/
│       ├── models.go         # Models with tenant_id fields
│       └── registry.go       # Model registration
└── go.mod
```

## Multi-Tenancy Patterns

### Pattern 1: Shared Database with tenant_id Column ✅ RECOMMENDED

All tenants share the same database, but each row has a `tenant_id` column. The `TenantDB` wrapper automatically injects tenant filters into all queries.

**Pros:**
- Simple to set up and maintain
- Efficient resource usage (one database)
- Easy to query across tenants (analytics, admin features)
- Cost-effective for large numbers of tenants

**Cons:**
- Requires careful filter management to prevent data leaks
- All tenants share database resources
- Cannot restore individual tenant data easily

**Implementation:**

```go
// Models include tenant_id
type User struct {
    ID       string `po:"id,primaryKey,uuid"`
    TenantID string `po:"tenant_id,uuid,notNull,index"`
    Name     string `po:"name,varchar(255),notNull"`
    Email    string `po:"email,varchar(320),notNull"`
}

// Create tenant-aware wrapper
tenantDB := database.NewTenantDB(qb, "tenant-123")

// All queries automatically filtered by tenant_id
users, err := database.Select[models.User](tenantDB).
    Where(builder.Gte("age", 18)).
    All(ctx)
// SQL: SELECT * FROM users WHERE tenant_id = 'tenant-123' AND age >= 18
```

### Pattern 2: Database-per-Tenant 🔒 MAXIMUM ISOLATION

Each tenant has their own PostgreSQL database with complete data isolation.

**Pros:**
- Perfect data isolation at database level
- Independent backups and migrations per tenant
- Easy to scale specific tenants
- Regulatory compliance (data residency)

**Cons:**
- More complex infrastructure
- Higher resource usage (multiple connection pools)
- Harder to query across tenants
- May not scale beyond hundreds/thousands of tenants

**Implementation:**

```go
// Create tenant manager
tm := database.NewTenantManager()
defer tm.CloseAll()

// Get connection for specific tenant
// Creates/connects to database 'tenant_acme'
acmeDB, err := tm.GetConnection(ctx, "acme")

// Use standard query builders (no tenant_id needed)
qb := builder.New(acmeDB)
users, err := builder.Select[models.User](qb).All(ctx)
```

## Features Demonstrated

### Shared Database Pattern
- ✅ **Automatic Tenant Filtering**: TenantDB wrapper auto-injects tenant_id filters
- ✅ **Security**: Prevents accidental data leaks across tenants
- ✅ **Models with tenant_id**: User and Document models
- ✅ **Tenant Management**: Tenant model for storing tenant metadata
- ✅ **INSERT/SELECT/UPDATE/DELETE**: All operations tenant-scoped
- ✅ **COUNT**: Tenant-scoped aggregations
- ✅ **Verification**: Examples showing data isolation

### Database-per-Tenant Pattern
- ✅ **Connection Manager**: TenantManager for managing multiple connections
- ✅ **Lazy Connection**: Connections created on-demand
- ✅ **Connection Pooling**: Each tenant gets their own pool
- ✅ **Thread-Safe**: Concurrent access to tenant connections
- ✅ **Cleanup**: Proper connection cleanup

## Prerequisites

- Go 1.24+
- PostgreSQL 12+

## Setup

### 1. Create Database

```bash
createdb pebble_multitenancy
```

### 2. Run Migrations (Optional)

```sql
-- Create tables
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    subdomain VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(320) NOT NULL,
    role VARCHAR(50) DEFAULT 'user' NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_users_tenant_id ON users(tenant_id);

CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id),
    is_public BOOLEAN DEFAULT false NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_documents_tenant_id ON documents(tenant_id);
```

### 3. Set Environment Variable (Optional)

```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/pebble_multitenancy?sslmode=disable"
```

### 4. Run Example

```bash
cd examples/multi-tenancy
go run cmd/multi-tenancy/main.go
```

## How It Works

### Shared Database Pattern

#### 1. TenantDB Wrapper

The `TenantDB` type wraps the query builder and automatically injects tenant filters:

```go
type TenantDB struct {
    qb       *builder.DB
    tenantID string
}

func Select[T any](tdb *TenantDB) *builder.SelectQuery[T] {
    query := builder.Select[T](tdb.qb)
    if hasTenantIDColumn[T]() {
        query = query.Where(builder.Eq("tenant_id", tdb.tenantID))
    }
    return query
}
```

#### 2. Auto-Filtering

All queries through `TenantDB` are automatically scoped:

```go
// Create tenant-scoped wrapper
acmeDB := database.NewTenantDB(qb, "acme-tenant-id")

// Query automatically filtered
users, _ := database.Select[models.User](acmeDB).
    Where(builder.Eq("role", "admin")).
    All(ctx)

// Generated SQL:
// SELECT * FROM users
// WHERE tenant_id = 'acme-tenant-id' AND role = 'admin'
```

#### 3. INSERT Requires Manual tenant_id

You must set `tenant_id` when creating records:

```go
user := models.User{
    TenantID: acmeDB.GetTenantID(),  // Get from wrapper
    Name:     "Alice",
    Email:    "alice@acme.com",
}
builder.Insert[models.User](qb).Values(user).Exec(ctx)
```

### Database-per-Tenant Pattern

#### 1. TenantManager

Manages multiple database connections:

```go
tm := database.NewTenantManager()

// Each tenant gets their own database and connection pool
acmeDB, err := tm.GetConnection(ctx, "acme")
widgetDB, err := tm.GetConnection(ctx, "widget")
```

#### 2. Connection Configuration

Connections created with tenant-specific database names:

```go
config := &runtime.Config{
    Host:     "localhost",
    Port:     5432,
    Database: fmt.Sprintf("tenant_%s", tenantID),  // tenant_acme, tenant_widget, etc.
    User:     "postgres",
    Password: "postgres",
    MaxConns: 10,
    MinConns: 2,
}
```

#### 3. Standard Queries

No tenant filtering needed (database-level isolation):

```go
qb := builder.New(acmeDB)
users, err := builder.Select[models.User](qb).All(ctx)
// Only returns acme's data (different database)
```

## Expected Output

```
=== Pebble ORM Multi-Tenancy Examples ===

>>> Pattern 1: Shared Database with tenant_id Column <<<

--- Setup: Creating Tenants ---
Created tenant: Acme Corp (ID: uuid-1)
Created tenant: Widget Inc (ID: uuid-2)

--- Example 1: INSERT Users (Tenant-Scoped) ---
[Acme] Created user: Alice Johnson (alice@acme.com)
[Widget] Created user: Bob Smith (bob@widget.com)

--- Example 2: SELECT with Auto Tenant Filtering ---
[Acme] Found 1 users:
  - Alice Johnson (alice@acme.com)
[Widget] Found 1 users:
  - Bob Smith (bob@widget.com)

--- Example 3: INSERT Documents (Tenant-Scoped) ---
[Acme] Created document: Acme Q1 Report
[Widget] Created document: Widget Product Roadmap

--- Example 4: SELECT Documents (Tenant Isolated) ---
[Acme] Found 1 documents:
  - Acme Q1 Report
[Widget] Found 1 documents:
  - Widget Product Roadmap

--- Example 5: UPDATE with Tenant Filtering ---
[Acme] Updated 1 user(s) to super_admin

--- Example 6: COUNT (Tenant-Scoped) ---
[Acme] Total users: 1
[Widget] Total users: 1

--- Example 7: Verify Tenant Isolation ---
✓ Each tenant wrapper automatically filters by tenant_id
✓ Acme cannot see Widget's data and vice versa
✓ All SELECT, UPDATE, DELETE queries are tenant-scoped
✓ INSERT requires manually setting tenant_id on the model

>>> Pattern 2: Database-per-Tenant (Conceptual) <<<

Benefits:
  ✓ Perfect data isolation (separate databases)
  ✓ No need for tenant_id columns
  ✓ Easy per-tenant backups and migrations
  ✓ Scales well with moderate number of tenants

=== All Examples Complete ===
```

## Security Considerations

### Shared Database Pattern

1. **Always use TenantDB wrapper** - Never use raw query builder for tenant data
2. **Validate tenant_id on INSERT** - Always set it explicitly
3. **Audit queries** - Log all queries to verify tenant filtering
4. **Testing** - Thoroughly test tenant isolation
5. **Admin operations** - Create separate admin queries that bypass wrapper when needed

### Database-per-Tenant Pattern

1. **Connection management** - Ensure connections are properly scoped per request
2. **Database creation** - Secure the database provisioning process
3. **Connection limits** - Monitor total connections across all tenants
4. **Backup strategy** - Automated backups for each tenant database

## When to Use Each Pattern

### Use Shared Database When:
- You have many tenants (100s to 1000s+)
- Resource efficiency is important
- Need cross-tenant analytics
- Simple backup/restore requirements
- Homogeneous tenant sizes

### Use Database-per-Tenant When:
- Strict data isolation required
- Regulatory compliance needs
- Per-tenant customization (schema, extensions)
- Independent backup/restore critical
- Moderate number of tenants (<100)
- Tenants vary significantly in size

## Hybrid Approach

You can combine both patterns:
- Small tenants → Shared database
- Large/enterprise tenants → Dedicated databases
- Use tenant metadata to route requests appropriately

## Performance Tips

1. **Indexes**: Always index `tenant_id` columns
2. **Connection Pools**: Size pools appropriately per pattern
3. **Caching**: Cache tenant metadata to avoid repeated lookups
4. **Prepared Statements**: Leverage pgx's prepared statement cache
5. **Monitoring**: Track queries per tenant for anomaly detection

## Related Examples

- [Basic Example](../basic) - Core CRUD operations
- [Transactions Example](../transactions) - Transaction handling
- [Relationships Example](../relationships) - Working with related data

## Further Reading

- [Multi-Tenancy Architecture Patterns](https://docs.microsoft.com/en-us/azure/architecture/guide/multitenant/)
- [PostgreSQL Row-Level Security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
- [Designing Multi-Tenant Applications](https://www.citusdata.com/blog/2016/10/03/designing-your-saas-database-for-high-scalability/)
