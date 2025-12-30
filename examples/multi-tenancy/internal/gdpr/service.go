package gdpr

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/marshallshelly/pebble-orm/examples/multi-tenancy/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Service handles GDPR compliance operations
type Service struct {
	db *builder.DB
}

// NewService creates a new GDPR service
func NewService(db *builder.DB) *Service {
	return &Service{db: db}
}

// LogAudit creates an audit log entry for GDPR compliance
func (s *Service) LogAudit(ctx context.Context, tenantID, userID, action, resource, resourceID, ipAddress, userAgent string, changes interface{}) error {
	log := models.AuditLog{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	}

	// Convert changes to JSONB if provided
	if changes != nil {
		changesJSON, err := json.Marshal(changes)
		if err != nil {
			return fmt.Errorf("failed to marshal changes: %w", err)
		}
		var jsonbData map[string]interface{}
		if err := json.Unmarshal(changesJSON, &jsonbData); err != nil {
			return fmt.Errorf("failed to unmarshal to JSONB: %w", err)
		}
		log.Changes = schema.JSONB(jsonbData)
	}

	_, err := builder.Insert[models.AuditLog](s.db).
		Values(log).
		Exec(ctx)

	return err
}

// SoftDeleteUser performs a soft delete (GDPR Article 17 - Right to Erasure)
// Keeps data for audit trail but marks as deleted
func (s *Service) SoftDeleteUser(ctx context.Context, tenantID, userID, deletedBy, ipAddress, userAgent string) error {
	now := time.Now()

	// Get user before deletion for audit log
	users, err := builder.Select[models.User](s.db).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		And(builder.IsNull("deleted_at")).
		All(ctx)

	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("user not found or already deleted")
	}

	// Soft delete the user
	_, err = builder.Update[models.User](s.db).
		Set("deleted_at", now).
		Set("deleted_by", deletedBy).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		And(builder.IsNull("deleted_at")).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("soft delete failed: %w", err)
	}

	// Log the deletion
	changes := map[string]interface{}{
		"deleted_at": now,
		"deleted_by": deletedBy,
	}

	return s.LogAudit(ctx, tenantID, deletedBy, "DELETE", "users", userID, ipAddress, userAgent, changes)
}

// AnonymizeUser replaces personal data with anonymized values
// (GDPR Recital 26 - Anonymization)
// Used when legal retention requires keeping records but not personal data
func (s *Service) AnonymizeUser(ctx context.Context, tenantID, userID, anonymizedBy, ipAddress, userAgent string) error {
	// Get user before anonymization for audit log
	users, err := builder.Select[models.User](s.db).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		All(ctx)

	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("user not found")
	}

	user := users[0]

	// Store original data in changes
	changes := map[string]interface{}{
		"original_name":  user.Name,
		"original_email": user.Email,
		"anonymized_at":  time.Now(),
		"anonymized_by":  anonymizedBy,
	}

	// Anonymize personal data
	anonymizedEmail := fmt.Sprintf("anonymized_%s@deleted.local", userID)
	_, err = builder.Update[models.User](s.db).
		Set("name", "Anonymized User").
		Set("email", anonymizedEmail).
		Set("phone", nil).
		Set("anonymized", true).
		Set("updated_by", anonymizedBy).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("anonymize failed: %w", err)
	}

	// Log the anonymization
	return s.LogAudit(ctx, tenantID, anonymizedBy, "ANONYMIZE", "users", userID, ipAddress, userAgent, changes)
}

// HardDeleteUser permanently deletes user and all related data
// Use with extreme caution - irreversible!
// Only use after soft delete and retention period has passed
func (s *Service) HardDeleteUser(ctx context.Context, tenantID, userID string) error {
	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete user's documents
	_, err = builder.Delete[models.Document](s.db).
		Where(builder.Eq("owner_id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete documents failed: %w", err)
	}

	// Delete data export requests
	_, err = builder.Delete[models.DataExportRequest](s.db).
		Where(builder.Eq("user_id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete export requests failed: %w", err)
	}

	// Delete deletion requests
	_, err = builder.Delete[models.DeletionRequest](s.db).
		Where(builder.Eq("user_id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete deletion requests failed: %w", err)
	}

	// Delete the user (audit logs are kept for compliance)
	_, err = builder.Delete[models.User](s.db).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete user failed: %w", err)
	}

	return tx.Commit()
}

// ExportUserData exports all user data in structured format
// (GDPR Article 20 - Right to Data Portability)
func (s *Service) ExportUserData(ctx context.Context, tenantID, userID string) (map[string]interface{}, error) {
	export := make(map[string]interface{})

	// Export user profile
	users, err := builder.Select[models.User](s.db).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("export user failed: %w", err)
	}
	if len(users) > 0 {
		export["user"] = users[0]
	}

	// Export user's documents
	docs, err := builder.Select[models.Document](s.db).
		Where(builder.Eq("owner_id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		And(builder.IsNull("deleted_at")).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("export documents failed: %w", err)
	}
	export["documents"] = docs

	// Export consent history from audit logs
	logs, err := builder.Select[models.AuditLog](s.db).
		Where(builder.Eq("user_id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		And(builder.Eq("action", "CONSENT_UPDATE")).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("export consent history failed: %w", err)
	}
	export["consent_history"] = logs

	// Export metadata
	export["exported_at"] = time.Now().Format(time.RFC3339)
	export["format_version"] = "1.0"
	export["tenant_id"] = tenantID
	export["user_id"] = userID

	return export, nil
}

// CreateDataExportRequest creates a data export request for a user
// (GDPR Article 20 - Right to Data Portability)
func (s *Service) CreateDataExportRequest(ctx context.Context, tenantID, userID, requestedBy string) (*models.DataExportRequest, error) {
	expiresAt := time.Now().AddDate(0, 0, 30) // Expire after 30 days

	request := models.DataExportRequest{
		TenantID:  tenantID,
		UserID:    userID,
		Status:    "pending",
		Format:    "json",
		ExpiresAt: &expiresAt,
		GDPRMetadata: models.GDPRMetadata{
			CreatedBy: &requestedBy,
		},
	}

	requests, err := builder.Insert[models.DataExportRequest](s.db).
		Values(request).
		Returning("*").
		ExecReturning(ctx)

	if err != nil {
		return nil, fmt.Errorf("create export request failed: %w", err)
	}

	if len(requests) == 0 {
		return nil, fmt.Errorf("no export request returned")
	}

	return &requests[0], nil
}

// CreateDeletionRequest creates a deletion request for a user
// (GDPR Article 17 - Right to Erasure)
func (s *Service) CreateDeletionRequest(ctx context.Context, tenantID, userID, requestedBy string) (*models.DeletionRequest, error) {
	request := models.DeletionRequest{
		TenantID:    tenantID,
		UserID:      userID,
		Status:      "pending",
		RequestType: "user",
		GDPRMetadata: models.GDPRMetadata{
			CreatedBy: &requestedBy,
		},
	}

	requests, err := builder.Insert[models.DeletionRequest](s.db).
		Values(request).
		Returning("*").
		ExecReturning(ctx)

	if err != nil {
		return nil, fmt.Errorf("create deletion request failed: %w", err)
	}

	if len(requests) == 0 {
		return nil, fmt.Errorf("no deletion request returned")
	}

	return &requests[0], nil
}

// UpdateConsent records user consent changes
// (GDPR Article 7 - Conditions for consent)
func (s *Service) UpdateConsent(ctx context.Context, tenantID, userID, consentType, updatedBy, ipAddress, userAgent string, granted bool) error {
	now := time.Now()

	var column, timestampColumn string
	switch consentType {
	case "marketing":
		column = "marketing_consent"
		timestampColumn = "marketing_consent_at"
	case "analytics":
		column = "analytics_consent"
		timestampColumn = "analytics_consent_at"
	default:
		return fmt.Errorf("invalid consent type: %s", consentType)
	}

	// Update consent
	_, err := builder.Update[models.User](s.db).
		Set(column, granted).
		Set(timestampColumn, now).
		Set("updated_by", updatedBy).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("update consent failed: %w", err)
	}

	// Log consent change
	changes := map[string]interface{}{
		"consent_type": consentType,
		"granted":      granted,
		"timestamp":    now.Format(time.RFC3339),
	}

	return s.LogAudit(ctx, tenantID, userID, "CONSENT_UPDATE", "users", userID, ipAddress, userAgent, changes)
}

// CleanupExpiredData removes data past retention period
// (GDPR Article 5 - Storage limitation)
func (s *Service) CleanupExpiredData(ctx context.Context, tenantID string) error {
	// Get tenant's retention policy
	tenants, err := builder.Select[models.Tenant](s.db).
		Where(builder.Eq("id", tenantID)).
		All(ctx)
	if err != nil || len(tenants) == 0 {
		return fmt.Errorf("tenant not found")
	}

	tenant := tenants[0]
	cutoffDate := time.Now().AddDate(0, 0, -tenant.DataRetentionDays)

	// Anonymize users past retention (if not already anonymized)
	_, err = builder.Update[models.User](s.db).
		Set("name", "Anonymized User").
		Set("email", "anonymized@deleted.local").
		Set("phone", nil).
		Set("anonymized", true).
		Where(builder.Eq("tenant_id", tenantID)).
		And(builder.Lt("created_at", cutoffDate)).
		And(builder.Eq("anonymized", false)).
		And(builder.IsNotNull("deleted_at")). // Only anonymize soft-deleted users
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("cleanup expired data failed: %w", err)
	}

	// Log cleanup action
	changes := map[string]interface{}{
		"cutoff_date":       cutoffDate.Format(time.RFC3339),
		"retention_days":    tenant.DataRetentionDays,
		"cleanup_timestamp": time.Now().Format(time.RFC3339),
	}

	return s.LogAudit(ctx, tenantID, "system", "CLEANUP", "users", "batch", "system", "gdpr-cleanup", changes)
}
