# Multi-Tenancy

<em>One database, many tenants, GDPR paperwork included.</em>

A REST API (Go Fiber) using the shared-database-with-`tenant_id` pattern. A `TenantDB` wrapper injects the tenant filter into every query, an embedded `GDPRMetadata` struct gives every table soft delete and audit fields, and a `gdpr.Service` implements the compliance operations: consent tracking, audit logging, data export, anonymization, erasure.

## Run

```bash
createdb pebble_multitenancy
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/pebble_multitenancy?sslmode=disable"

cd examples/multi-tenancy
go mod tidy
pebble generate --name initial_schema --models ./internal/models
pebble migrate up --all --db "$DATABASE_URL"
go run cmd/multi-tenancy/main.go        # serves on :3000, override with PORT
```

## What it shows

| Feature | Where |
|---------|-------|
| Embedded GDPR fields on every model (`created_by`, `deleted_at`, `retain_until`, `anonymized`, …) | `internal/models/models.go` |
| Tenant-scoped query wrapper (`SelectActive`, `SelectAll`, `Update`, `Delete`) | `internal/database/db.go` |
| Database-per-tenant connection manager (`TenantManager`) as the alternative pattern | `internal/database/db.go` |
| Consent management, audit logging, soft delete, anonymization, data export, hard delete in a transaction | `internal/gdpr/service.go` |
| Fiber handlers wiring it all to REST endpoints | `cmd/multi-tenancy/main.go` |
| `schema.JSONB` column for before/after change diffs in audit logs | `AuditLog.Changes` |

## Tenant isolation

Every model that belongs to a tenant carries the ID; the wrapper adds the filters so handlers can't forget them:

```go
tenantDB := database.NewTenantDB(qb, tenantID, userID)

// WHERE tenant_id = $1 AND deleted_at IS NULL — injected automatically
users, err := database.SelectActive[models.User](tenantDB).
    OrderByAsc("created_at").
    All(ctx)

// Admin/audit view: same tenant filter, includes soft-deleted rows
all, err := database.SelectAll[models.User](tenantDB).All(ctx)
```

## GDPR metadata by embedding

One struct, embedded in every model, and the columns come along for free:

```go
type GDPRMetadata struct {
    CreatedAt   time.Time  `po:"created_at,timestamptz,default(NOW()),notNull"`
    CreatedBy   *string    `po:"created_by,uuid"`
    UpdatedAt   time.Time  `po:"updated_at,timestamptz,default(NOW()),notNull"`
    UpdatedBy   *string    `po:"updated_by,uuid"`
    DeletedAt   *time.Time `po:"deleted_at,timestamptz"`
    DeletedBy   *string    `po:"deleted_by,uuid"`
    RetainUntil *time.Time `po:"retain_until,timestamptz"`
    Anonymized  bool       `po:"anonymized,boolean,default(false),notNull"`
}

// table_name: users
type User struct {
    ID       string `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
    TenantID string `po:"tenant_id,uuid,notNull,index"`
    // ... personal data, consent flags, processing basis ...
    GDPRMetadata
}
```

Erasure comes in three escalating flavors in `gdpr/service.go`: `SoftDeleteUser` (sets `deleted_at`, keeps the audit trail), `AnonymizeUser` (rewrites name/email/phone in place, keeps the row), and `HardDeleteUser` (transactional cascade delete — audit logs survive).

## Endpoints

```
POST   /api/v1/tenants                                            create tenant
GET    /api/v1/tenants/:id
POST   /api/v1/tenants/:tenantId/users                            create user with consent flags
GET    /api/v1/tenants/:tenantId/users                            list (excludes soft-deleted)
GET    /api/v1/tenants/:tenantId/users/:userId                    read (access is audit-logged)
PUT    /api/v1/tenants/:tenantId/users/:userId                    update (diff is audit-logged)
PUT    /api/v1/tenants/:tenantId/users/:userId/consent            Article 7
GET    /api/v1/tenants/:tenantId/users/:userId/audit-logs         Article 15
POST   /api/v1/tenants/:tenantId/users/:userId/export             Article 20
GET    /api/v1/tenants/:tenantId/export-requests/:requestId
POST   /api/v1/tenants/:tenantId/users/:userId/delete             Article 17 (request)
DELETE /api/v1/tenants/:tenantId/users/:userId/soft               soft delete
DELETE /api/v1/tenants/:tenantId/users/:userId/anonymize          anonymize in place
POST   /api/v1/tenants/:tenantId/documents                        tenant-scoped documents
GET    /api/v1/tenants/:tenantId/documents[/:documentId]
```

The caller identity comes from the `X-User-ID` header (stand-in for real auth — wire up JWT/OAuth before shipping anything like this).

<details>
<summary><strong>curl walkthrough</strong></summary>

```bash
# 1. Create a tenant with GDPR configuration
curl -X POST http://localhost:3000/api/v1/tenants \
  -H "Content-Type: application/json" \
  -d '{"name":"Acme Corp","subdomain":"acme","data_region":"EU","data_retention_days":365,"dpo_email":"dpo@acme.com"}'

# 2. Create a user with consent (use the tenant id from step 1)
curl -X POST http://localhost:3000/api/v1/tenants/$TENANT_ID/users \
  -H "Content-Type: application/json" -H "X-User-ID: admin" \
  -d '{"name":"Alice Johnson","email":"alice@acme.com","marketing_consent":true,"processing_basis":"consent"}'

# 3. Withdraw marketing consent
curl -X PUT http://localhost:3000/api/v1/tenants/$TENANT_ID/users/$USER_ID/consent \
  -H "Content-Type: application/json" -H "X-User-ID: $USER_ID" \
  -d '{"consent_type":"marketing","granted":false}'

# 4. Export everything Pebble knows about the user (Article 20)
curl -X POST http://localhost:3000/api/v1/tenants/$TENANT_ID/users/$USER_ID/export \
  -H "X-User-ID: $USER_ID"

# 5. Soft delete, then check the audit trail
curl -X DELETE http://localhost:3000/api/v1/tenants/$TENANT_ID/users/$USER_ID/soft -H "X-User-ID: admin"
curl http://localhost:3000/api/v1/tenants/$TENANT_ID/users/$USER_ID/audit-logs
```

</details>

**Not legal advice.** This is a technical pattern reference — GDPR compliance also requires organizational measures (DPO, privacy policy, DPAs, DPIAs). Talk to a lawyer.
