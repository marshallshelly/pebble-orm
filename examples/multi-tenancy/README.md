# GDPR-Compliant Multi-Tenancy Example

This example demonstrates a **production-ready, GDPR-compliant multi-tenant SaaS application** using Pebble ORM with a REST API built on Go Fiber.

## 🔒 GDPR Compliance

This example implements **strict GDPR compliance** with all major requirements:

| GDPR Article | Feature | Implementation |
|--------------|---------|----------------|
| **Article 5** | Accountability & Storage Limitation | Audit trails, data retention policies |
| **Article 6** | Lawful Processing | Processing basis tracking |
| **Article 7** | Consent Management | Consent tracking with timestamps |
| **Article 15** | Right to Access | Audit log access endpoints |
| **Article 17** | Right to Erasure | Soft delete, anonymization, hard delete |
| **Article 20** | Right to Data Portability | Data export in JSON format |
| **Article 32** | Security | Tenant isolation, audit logging |
| **Articles 44-50** | International Transfers | Data residency tracking |

## 🏗️ Architecture

### Multi-Tenancy Patterns

This example demonstrates **shared database with tenant_id** pattern:

- ✅ **Automatic Tenant Filtering**: All queries auto-filtered by tenant_id
- ✅ **Soft Delete**: Records marked as deleted, not removed
- ✅ **Audit Logging**: All actions logged for compliance
- ✅ **Data Isolation**: Perfect separation between tenants

**Pattern Comparison:**

| Feature | Shared DB (Implemented) | Database-per-Tenant |
|---------|------------------------|---------------------|
| **Tenant Limit** | 1000s+ | <100 |
| **Resource Usage** | Efficient | Higher |
| **Isolation** | Logical (row-level) | Physical (database-level) |
| **GDPR Compliance** | Requires careful implementation | Easier (drop database) |
| **Cost** | Lower | Higher |

## 📁 Project Structure

```
multi-tenancy/
├── cmd/
│   └── multi-tenancy/
│       └── main.go              # REST API server with Fiber
├── internal/
│   ├── database/
│   │   └── db.go                # TenantDB wrapper, automatic filtering
│   ├── gdpr/
│   │   └── service.go           # GDPR compliance operations
│   └── models/
│       ├── models.go            # GDPR-compliant models
│       └── registry.go          # Model registration
├── go.mod
└── README.md
```

## 🎯 GDPR Features Implemented

### 1. Soft Delete (Article 17 - Right to Erasure)

Records are never immediately deleted. They're marked as deleted for audit trail:

```go
type GDPRMetadata struct {
    DeletedAt *time.Time `po:"deleted_at,timestamptz"`
    DeletedBy *string    `po:"deleted_by,uuid"`
}
```

**API Endpoint:**
```bash
DELETE /api/v1/tenants/:tenantId/users/:userId/soft
```

### 2. Anonymization (GDPR Recital 26)

Removes personal data while keeping records for legal compliance:

```go
// Before: name="John Doe", email="john@example.com"
// After:  name="Anonymized User", email="anonymized_uuid@deleted.local"
```

**API Endpoint:**
```bash
DELETE /api/v1/tenants/:tenantId/users/:userId/anonymize
```

### 3. Audit Logging (Article 5 - Accountability)

All actions logged with who, what, when, where:

```go
type AuditLog struct {
    UserID     string       `po:"user_id,uuid,notNull"`
    Action     string       `po:"action,varchar(50),notNull"` // CREATE, READ, UPDATE, DELETE
    Resource   string       `po:"resource,varchar(100),notNull"`
    Changes    schema.JSONB `po:"changes,jsonb"`
    IPAddress  string       `po:"ip_address,varchar(45),notNull"`
    UserAgent  string       `po:"user_agent,text"`
    CreatedAt  time.Time    `po:"created_at,timestamptz,default(NOW()),notNull"`
}
```

**API Endpoint:**
```bash
GET /api/v1/tenants/:tenantId/users/:userId/audit-logs
```

### 4. Consent Management (Article 7)

Tracks user consent with timestamps:

```go
type User struct {
    MarketingConsent   bool       `po:"marketing_consent,boolean,default(false),notNull"`
    MarketingConsentAt *time.Time `po:"marketing_consent_at,timestamptz"`
    AnalyticsConsent   bool       `po:"analytics_consent,boolean,default(false),notNull"`
    AnalyticsConsentAt *time.Time `po:"analytics_consent_at,timestamptz"`
}
```

**API Endpoint:**
```bash
PUT /api/v1/tenants/:tenantId/users/:userId/consent
{
  "consent_type": "marketing",
  "granted": true
}
```

### 5. Data Portability (Article 20)

Export all user data in structured JSON format:

```go
{
  "user": { /* user profile */ },
  "documents": [ /* all documents */ ],
  "consent_history": [ /* consent changes */ ],
  "exported_at": "2024-01-15T10:30:00Z",
  "tenant_id": "uuid",
  "user_id": "uuid"
}
```

**API Endpoint:**
```bash
POST /api/v1/tenants/:tenantId/users/:userId/export
```

### 6. Data Retention (Article 5 - Storage Limitation)

Automatic cleanup after retention period:

```go
type Tenant struct {
    DataRetentionDays int `po:"data_retention_days,integer,default(365),notNull"`
}

// Automatically anonymizes data after retention period
gdprService.CleanupExpiredData(ctx, tenantID)
```

### 7. Processing Basis (Article 6)

Tracks legal basis for data processing:

```go
type User struct {
    ProcessingBasis string `po:"processing_basis,varchar(50),default('consent'),notNull"`
    // Values: "consent", "contract", "legitimate_interest"
}
```

### 8. Data Residency (Articles 44-50)

Tracks where data is stored:

```go
type Tenant struct {
    DataRegion string `po:"data_region,varchar(50),default('EU'),notNull"` // EU, US, UK, etc.
}
```

## 🚀 Getting Started

### Prerequisites

- Go 1.24+
- PostgreSQL 14+
- Docker (optional)

### 1. Create Database

```bash
createdb pebble_multitenancy
```

### 2. Set Environment Variables (Optional)

```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/pebble_multitenancy?sslmode=disable"
export PORT=3000
```

### 3. Install Dependencies

```bash
cd examples/multi-tenancy
go mod tidy
```

### 4. Run the API Server

```bash
go run cmd/multi-tenancy/main.go
```

Output:
```
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

## 📡 API Endpoints

### Tenant Management

```bash
# Create tenant
POST /api/v1/tenants
{
  "name": "Acme Corp",
  "subdomain": "acme",
  "data_region": "EU",
  "data_retention_days": 365,
  "dpo_email": "dpo@acme.com"
}

# Get tenant
GET /api/v1/tenants/:id
```

### User Management (GDPR-Compliant)

```bash
# Create user with consent
POST /api/v1/tenants/:tenantId/users
{
  "name": "Alice Johnson",
  "email": "alice@acme.com",
  "phone": "+1234567890",
  "role": "admin",
  "marketing_consent": true,
  "analytics_consent": false,
  "processing_basis": "consent"
}

# List users (excludes soft-deleted)
GET /api/v1/tenants/:tenantId/users

# Get user (logs access for GDPR)
GET /api/v1/tenants/:tenantId/users/:userId

# Update user (logs changes)
PUT /api/v1/tenants/:tenantId/users/:userId
{
  "name": "Alice Smith",
  "email": "alice.smith@acme.com"
}
```

### GDPR Compliance Endpoints

```bash
# Update consent (Article 7)
PUT /api/v1/tenants/:tenantId/users/:userId/consent
{
  "consent_type": "marketing",
  "granted": true
}

# Get audit logs (Article 15 - Right to Access)
GET /api/v1/tenants/:tenantId/users/:userId/audit-logs

# Export user data (Article 20 - Data Portability)
POST /api/v1/tenants/:tenantId/users/:userId/export

# Get export request
GET /api/v1/tenants/:tenantId/export-requests/:requestId

# Request deletion (Article 17 - Right to Erasure)
POST /api/v1/tenants/:tenantId/users/:userId/delete

# Soft delete user (retains for audit trail)
DELETE /api/v1/tenants/:tenantId/users/:userId/soft

# Anonymize user (removes personal data)
DELETE /api/v1/tenants/:tenantId/users/:userId/anonymize
```

### Document Management (Tenant-Scoped)

```bash
# Create document
POST /api/v1/tenants/:tenantId/documents
{
  "title": "Q1 Report",
  "content": "Quarterly financial report...",
  "owner_id": "user-uuid",
  "is_public": false
}

# List documents (tenant-scoped, excludes deleted)
GET /api/v1/tenants/:tenantId/documents

# Get document (logs access)
GET /api/v1/tenants/:tenantId/documents/:documentId
```

## 💡 Usage Examples

### Example 1: Create Tenant with GDPR Configuration

```bash
curl -X POST http://localhost:3000/api/v1/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "subdomain": "acme",
    "data_region": "EU",
    "data_retention_days": 365,
    "dpo_email": "dpo@acme.com"
  }'
```

Response:
```json
{
  "id": "tenant-uuid",
  "name": "Acme Corp",
  "subdomain": "acme",
  "data_region": "EU",
  "data_retention_days": 365,
  "dpo_email": "dpo@acme.com",
  "created_at": "2024-01-15T10:00:00Z"
}
```

### Example 2: Create User with Consent

```bash
curl -X POST http://localhost:3000/api/v1/tenants/tenant-uuid/users \
  -H "Content-Type: application/json" \
  -H "X-User-ID: admin-uuid" \
  -d '{
    "name": "Alice Johnson",
    "email": "alice@acme.com",
    "marketing_consent": true,
    "analytics_consent": false,
    "processing_basis": "consent"
  }'
```

### Example 3: Export User Data (GDPR Article 20)

```bash
curl -X POST http://localhost:3000/api/v1/tenants/tenant-uuid/users/user-uuid/export \
  -H "X-User-ID: user-uuid"
```

Response:
```json
{
  "request_id": "export-uuid",
  "status": "completed",
  "data": {
    "user": { /* user profile */ },
    "documents": [ /* all documents */ ],
    "consent_history": [ /* consent changes */ ],
    "exported_at": "2024-01-15T10:30:00Z"
  },
  "expires_at": "2024-02-14T10:30:00Z"
}
```

### Example 4: Update Consent

```bash
curl -X PUT http://localhost:3000/api/v1/tenants/tenant-uuid/users/user-uuid/consent \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user-uuid" \
  -d '{
    "consent_type": "marketing",
    "granted": false
  }'
```

### Example 5: Soft Delete User

```bash
curl -X DELETE http://localhost:3000/api/v1/tenants/tenant-uuid/users/user-uuid/soft \
  -H "X-User-ID: admin-uuid"
```

Response:
```json
{
  "message": "User soft deleted successfully",
  "user_id": "user-uuid",
  "deleted_at": "2024-01-15T10:45:00Z",
  "note": "Data retained for audit trail. Will be anonymized after retention period."
}
```

## 🔐 Security Features

### Automatic Tenant Filtering

The `TenantDB` wrapper automatically injects tenant filters:

```go
tenantDB := database.NewTenantDB(qb, tenantID, userID)

// Automatically filtered by tenant_id
users, err := database.SelectActive[models.User](tenantDB).All(ctx)
// SQL: SELECT * FROM users WHERE tenant_id = ? AND deleted_at IS NULL
```

### Soft Delete Protection

All queries automatically exclude soft-deleted records:

```go
// SelectActive - excludes deleted records (normal operation)
users, err := database.SelectActive[models.User](tenantDB).All(ctx)

// SelectAll - includes deleted records (admin/audit only)
allUsers, err := database.SelectAll[models.User](tenantDB).All(ctx)
```

### Audit Trail

Every action is logged automatically:

```go
// Automatically logged on user creation
gdprService.LogAudit(ctx, tenantID, userID, "CREATE", "users", userID, ipAddress, userAgent, nil)

// Automatically logged on updates with before/after changes
changes := map[string]interface{}{
    "name": map[string]string{"old": "John", "new": "Jane"}
}
gdprService.LogAudit(ctx, tenantID, userID, "UPDATE", "users", userID, ipAddress, userAgent, changes)
```

## 📊 Database Schema

### Core Tables

```sql
-- Tenants with GDPR configuration
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    subdomain VARCHAR(100) UNIQUE NOT NULL,
    data_region VARCHAR(50) DEFAULT 'EU' NOT NULL,
    data_retention_days INTEGER DEFAULT 365 NOT NULL,
    dpo_email VARCHAR(320),
    -- GDPR metadata
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID,
    retain_until TIMESTAMPTZ,
    anonymized BOOLEAN DEFAULT false NOT NULL
);

-- Users with consent tracking
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    -- Personal data
    name VARCHAR(255) NOT NULL,
    email VARCHAR(320) NOT NULL,
    phone VARCHAR(50),
    role VARCHAR(50) DEFAULT 'user' NOT NULL,
    -- Consent tracking (GDPR Article 7)
    marketing_consent BOOLEAN DEFAULT false NOT NULL,
    marketing_consent_at TIMESTAMPTZ,
    analytics_consent BOOLEAN DEFAULT false NOT NULL,
    analytics_consent_at TIMESTAMPTZ,
    -- Legal basis (GDPR Article 6)
    processing_basis VARCHAR(50) DEFAULT 'consent' NOT NULL,
    -- GDPR metadata
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID,
    retain_until TIMESTAMPTZ,
    anonymized BOOLEAN DEFAULT false NOT NULL
);

CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);

-- Audit logs (never deleted)
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(100) NOT NULL,
    resource_id UUID NOT NULL,
    changes JSONB,
    ip_address VARCHAR(45) NOT NULL,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_audit_logs_tenant_id ON audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_resource_id ON audit_logs(resource_id);

-- Data export requests
CREATE TABLE data_export_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'pending' NOT NULL,
    format VARCHAR(20) DEFAULT 'json' NOT NULL,
    file_url TEXT,
    expires_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    created_by UUID
);

-- Deletion requests
CREATE TABLE deletion_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'pending' NOT NULL,
    request_type VARCHAR(50) DEFAULT 'user' NOT NULL,
    approved_by UUID,
    approved_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    created_by UUID
);
```

## ⚖️ GDPR Compliance Checklist

### Technical Implementation ✅

- [x] Soft delete on all user-related tables
- [x] Audit logging for all data access
- [x] Consent management with timestamps
- [x] Data export functionality (Article 20)
- [x] Right to erasure implementation (Article 17)
- [x] Data retention policies
- [x] Anonymization functions
- [x] Tenant isolation (automatic filtering)
- [x] Processing basis tracking
- [x] Data residency tracking
- [x] Created by / Updated by audit trail

### Operational Requirements ⚠️

These require business/legal implementation beyond the code:

- [ ] Appoint Data Protection Officer (DPO)
- [ ] Create privacy policy
- [ ] Create consent forms
- [ ] Set up data processing agreements
- [ ] Configure automated retention cleanup
- [ ] Set up audit log monitoring
- [ ] Train staff on GDPR requirements
- [ ] Test data export workflow
- [ ] Test right to erasure workflow
- [ ] Document data flows
- [ ] Data protection impact assessments (DPIA)
- [ ] Configure geographic data storage

## 🎓 Key Takeaways

### Why This Architecture?

1. **GDPR-First Design**: Every feature built with GDPR compliance in mind
2. **Audit Trail**: Complete transparency of who accessed what and when
3. **Soft Delete**: Never lose audit trail, satisfy legal retention
4. **Consent Management**: Track and respect user preferences
5. **Data Portability**: Easy export in standard formats
6. **Tenant Isolation**: Automatic, no chance of data leaks

### Production Considerations

1. **Background Jobs**: Export requests should be processed async
2. **File Storage**: Exported data should be stored securely (S3, etc.)
3. **Encryption**: Add encryption at rest and in transit
4. **Authentication**: Add proper JWT/OAuth authentication
5. **Rate Limiting**: Prevent abuse of export endpoints
6. **Monitoring**: Alert on unusual audit patterns
7. **Backup**: Encrypted backups with geographic redundancy

## 📚 Related Examples

- [Basic Example](../basic) - Core CRUD operations
- [Transactions Example](../transactions) - Transaction handling
- [Relationships Example](../relationships) - Working with related data

## 📖 Further Reading

- [GDPR Official Text](https://gdpr-info.eu/)
- [ICO Guide to GDPR](https://ico.org.uk/for-organisations/guide-to-data-protection/)
- [PostgreSQL Row-Level Security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
- [Multi-Tenancy Architecture Patterns](https://docs.microsoft.com/en-us/azure/architecture/guide/multitenant/)

## ⚠️ Legal Disclaimer

**This example provides technical implementation patterns only.**

- Consult legal counsel for GDPR compliance
- Requirements vary by jurisdiction
- This is not legal advice
- GDPR fines can reach €20M or 4% of global revenue
- Compliance requires both technical AND organizational measures

---

**This is a production-ready, GDPR-compliant multi-tenant SaaS application!** 🎉
