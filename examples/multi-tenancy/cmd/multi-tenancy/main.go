package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/marshallshelly/pebble-orm/examples/multi-tenancy/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/multi-tenancy/internal/gdpr"
	"github.com/marshallshelly/pebble-orm/examples/multi-tenancy/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
)

type App struct {
	db          *builder.DB
	gdprService *gdpr.Service
}

func main() {
	ctx := context.Background()

	// Connect to database
	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	qb := builder.New(db)

	// Initialize app
	app := &App{
		db:          qb,
		gdprService: gdpr.NewService(qb),
	}

	// Create Fiber app
	fiberApp := fiber.New(fiber.Config{
		ErrorHandler: customErrorHandler,
	})

	// Middleware
	fiberApp.Use(logger.New())
	fiberApp.Use(recover.New())

	// Health check
	fiberApp.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	// API routes
	api := fiberApp.Group("/api/v1")

	// Tenant routes
	api.Post("/tenants", app.createTenant)
	api.Get("/tenants/:id", app.getTenant)

	// User routes (GDPR-compliant)
	api.Post("/tenants/:tenantId/users", app.createUser)
	api.Get("/tenants/:tenantId/users", app.listUsers)
	api.Get("/tenants/:tenantId/users/:userId", app.getUser)
	api.Put("/tenants/:tenantId/users/:userId", app.updateUser)

	// GDPR: Consent management (Article 7)
	api.Put("/tenants/:tenantId/users/:userId/consent", app.updateConsent)

	// GDPR: Right to Access (Article 15)
	api.Get("/tenants/:tenantId/users/:userId/audit-logs", app.getUserAuditLogs)

	// GDPR: Right to Data Portability (Article 20)
	api.Post("/tenants/:tenantId/users/:userId/export", app.requestDataExport)
	api.Get("/tenants/:tenantId/export-requests/:requestId", app.getExportRequest)

	// GDPR: Right to Erasure (Article 17)
	api.Post("/tenants/:tenantId/users/:userId/delete", app.requestDeletion)
	api.Delete("/tenants/:tenantId/users/:userId/soft", app.softDeleteUser)
	api.Delete("/tenants/:tenantId/users/:userId/anonymize", app.anonymizeUser)

	// Document routes (tenant-scoped)
	api.Post("/tenants/:tenantId/documents", app.createDocument)
	api.Get("/tenants/:tenantId/documents", app.listDocuments)
	api.Get("/tenants/:tenantId/documents/:documentId", app.getDocument)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Gracefully shutting down...")
		_ = fiberApp.Shutdown()
	}()

	log.Printf("🚀 GDPR-Compliant Multi-Tenant API Server starting on port %s", port)
	log.Printf("📋 API Documentation: http://localhost:%s/api/v1", port)
	log.Println("\n=== GDPR Features ===")
	log.Println("✅ Soft Delete (Article 17)")
	log.Println("✅ Audit Logging (Article 5)")
	log.Println("✅ Data Portability (Article 20)")
	log.Println("✅ Consent Management (Article 7)")
	log.Println("✅ Right to Erasure (Article 17)")
	log.Println("✅ Tenant Isolation")
	log.Println("")

	if err := fiberApp.Listen(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// Tenant handlers

func (app *App) createTenant(c *fiber.Ctx) error {
	var req struct {
		Name              string  `json:"name"`
		Subdomain         string  `json:"subdomain"`
		DataRegion        string  `json:"data_region"`
		DataRetentionDays int     `json:"data_retention_days"`
		DPOEmail          *string `json:"dpo_email"`
	}

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	tenant := models.Tenant{
		Name:              req.Name,
		Subdomain:         req.Subdomain,
		DataRegion:        req.DataRegion,
		DataRetentionDays: req.DataRetentionDays,
		DPOEmail:          req.DPOEmail,
	}

	if tenant.DataRegion == "" {
		tenant.DataRegion = "EU"
	}
	if tenant.DataRetentionDays == 0 {
		tenant.DataRetentionDays = 365
	}

	tenants, err := builder.Insert[models.Tenant](app.db).
		Values(tenant).
		Returning("*").
		ExecReturning(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to create tenant: %v", err))
	}

	return c.Status(fiber.StatusCreated).JSON(tenants[0])
}

func (app *App) getTenant(c *fiber.Ctx) error {
	tenantID := c.Params("id")

	tenants, err := builder.Select[models.Tenant](app.db).
		Where(builder.Eq("id", tenantID)).
		And(builder.IsNull("deleted_at")).
		All(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to get tenant: %v", err))
	}

	if len(tenants) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "Tenant not found")
	}

	return c.JSON(tenants[0])
}

// User handlers (GDPR-compliant)

func (app *App) createUser(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	createdBy := c.Get("X-User-ID", "system") // In production, extract from auth token

	var req struct {
		Name             string  `json:"name"`
		Email            string  `json:"email"`
		Phone            *string `json:"phone"`
		Role             string  `json:"role"`
		MarketingConsent bool    `json:"marketing_consent"`
		AnalyticsConsent bool    `json:"analytics_consent"`
		ProcessingBasis  string  `json:"processing_basis"`
	}

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	now := time.Now()
	user := models.User{
		TenantID:         tenantID,
		Name:             req.Name,
		Email:            req.Email,
		Phone:            req.Phone,
		Role:             req.Role,
		MarketingConsent: req.MarketingConsent,
		AnalyticsConsent: req.AnalyticsConsent,
		ProcessingBasis:  req.ProcessingBasis,
		GDPRMetadata: models.GDPRMetadata{
			CreatedBy: &createdBy,
		},
	}

	if user.Role == "" {
		user.Role = "user"
	}
	if user.ProcessingBasis == "" {
		user.ProcessingBasis = "consent"
	}

	// Set consent timestamps if granted
	if user.MarketingConsent {
		user.MarketingConsentAt = &now
	}
	if user.AnalyticsConsent {
		user.AnalyticsConsentAt = &now
	}

	users, err := builder.Insert[models.User](app.db).
		Values(user).
		Returning("*").
		ExecReturning(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to create user: %v", err))
	}

	// Log creation for GDPR audit trail
	_ = app.gdprService.LogAudit(c.Context(), tenantID, createdBy, "CREATE", "users", users[0].ID, c.IP(), c.Get("User-Agent"), nil)

	return c.Status(fiber.StatusCreated).JSON(users[0])
}

func (app *App) listUsers(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Get("X-User-ID", "system")

	tenantDB := database.NewTenantDB(app.db, tenantID, userID)

	users, err := database.SelectActive[models.User](tenantDB).
		OrderByAsc("created_at").
		All(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to list users: %v", err))
	}

	return c.JSON(users)
}

func (app *App) getUser(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Params("userId")
	requestedBy := c.Get("X-User-ID", "system")

	users, err := builder.Select[models.User](app.db).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		And(builder.IsNull("deleted_at")).
		All(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to get user: %v", err))
	}

	if len(users) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}

	// Log access for GDPR compliance (Article 15)
	_ = app.gdprService.LogAudit(c.Context(), tenantID, requestedBy, "READ", "users", userID, c.IP(), c.Get("User-Agent"), nil)

	return c.JSON(users[0])
}

func (app *App) updateUser(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Params("userId")
	updatedBy := c.Get("X-User-ID", "system")

	var req struct {
		Name  *string `json:"name"`
		Email *string `json:"email"`
		Phone *string `json:"phone"`
		Role  *string `json:"role"`
	}

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	// Get existing user for audit log
	existingUsers, err := builder.Select[models.User](app.db).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		All(c.Context())

	if err != nil || len(existingUsers) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}

	// Build update query
	query := builder.Update[models.User](app.db).
		Set("updated_by", updatedBy).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		And(builder.IsNull("deleted_at"))

	changes := make(map[string]interface{})

	if req.Name != nil {
		query = query.Set("name", *req.Name)
		changes["name"] = map[string]string{"old": existingUsers[0].Name, "new": *req.Name}
	}
	if req.Email != nil {
		query = query.Set("email", *req.Email)
		changes["email"] = map[string]string{"old": existingUsers[0].Email, "new": *req.Email}
	}
	if req.Phone != nil {
		query = query.Set("phone", *req.Phone)
	}
	if req.Role != nil {
		query = query.Set("role", *req.Role)
		changes["role"] = map[string]string{"old": existingUsers[0].Role, "new": *req.Role}
	}

	_, err = query.Exec(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to update user: %v", err))
	}

	// Log update for GDPR audit trail
	_ = app.gdprService.LogAudit(c.Context(), tenantID, updatedBy, "UPDATE", "users", userID, c.IP(), c.Get("User-Agent"), changes)

	// Get updated user
	users, _ := builder.Select[models.User](app.db).
		Where(builder.Eq("id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		All(c.Context())

	return c.JSON(users[0])
}

// GDPR: Consent Management (Article 7)

func (app *App) updateConsent(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Params("userId")
	updatedBy := c.Get("X-User-ID", userID) // User can update their own consent

	var req struct {
		ConsentType string `json:"consent_type"` // "marketing" or "analytics"
		Granted     bool   `json:"granted"`
	}

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	err := app.gdprService.UpdateConsent(c.Context(), tenantID, userID, req.ConsentType, updatedBy, c.IP(), c.Get("User-Agent"), req.Granted)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to update consent: %v", err))
	}

	return c.JSON(fiber.Map{
		"message":      "Consent updated successfully",
		"consent_type": req.ConsentType,
		"granted":      req.Granted,
		"updated_at":   time.Now(),
	})
}

// GDPR: Right to Access (Article 15)

func (app *App) getUserAuditLogs(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Params("userId")

	logs, err := builder.Select[models.AuditLog](app.db).
		Where(builder.Eq("user_id", userID)).
		And(builder.Eq("tenant_id", tenantID)).
		OrderByDesc("created_at").
		Limit(100).
		All(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to get audit logs: %v", err))
	}

	return c.JSON(fiber.Map{
		"user_id":    userID,
		"tenant_id":  tenantID,
		"audit_logs": logs,
		"count":      len(logs),
	})
}

// GDPR: Right to Data Portability (Article 20)

func (app *App) requestDataExport(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Params("userId")
	requestedBy := c.Get("X-User-ID", userID)

	// Create export request
	request, err := app.gdprService.CreateDataExportRequest(c.Context(), tenantID, userID, requestedBy)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to create export request: %v", err))
	}

	// In production, this would trigger a background job to generate the export
	// For demo, we'll export immediately
	exportData, err := app.gdprService.ExportUserData(c.Context(), tenantID, userID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to export data: %v", err))
	}

	// Update request status
	now := time.Now()
	_, _ = builder.Update[models.DataExportRequest](app.db).
		Set("status", "completed").
		Set("completed_at", now).
		Where(builder.Eq("id", request.ID)).
		Exec(c.Context())

	// Log export for GDPR audit trail
	_ = app.gdprService.LogAudit(c.Context(), tenantID, requestedBy, "EXPORT", "users", userID, c.IP(), c.Get("User-Agent"), nil)

	return c.JSON(fiber.Map{
		"request_id":   request.ID,
		"status":       "completed",
		"data":         exportData,
		"expires_at":   request.ExpiresAt,
		"completed_at": now,
	})
}

func (app *App) getExportRequest(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	requestID := c.Params("requestId")

	requests, err := builder.Select[models.DataExportRequest](app.db).
		Where(builder.Eq("id", requestID)).
		And(builder.Eq("tenant_id", tenantID)).
		All(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to get export request: %v", err))
	}

	if len(requests) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "Export request not found")
	}

	return c.JSON(requests[0])
}

// GDPR: Right to Erasure (Article 17)

func (app *App) requestDeletion(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Params("userId")
	requestedBy := c.Get("X-User-ID", userID)

	request, err := app.gdprService.CreateDeletionRequest(c.Context(), tenantID, userID, requestedBy)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to create deletion request: %v", err))
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"request_id":  request.ID,
		"status":      request.Status,
		"message":     "Deletion request created. Pending approval.",
		"created_at":  request.CreatedAt,
	})
}

func (app *App) softDeleteUser(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Params("userId")
	deletedBy := c.Get("X-User-ID", "system")

	err := app.gdprService.SoftDeleteUser(c.Context(), tenantID, userID, deletedBy, c.IP(), c.Get("User-Agent"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to soft delete user: %v", err))
	}

	return c.JSON(fiber.Map{
		"message":    "User soft deleted successfully",
		"user_id":    userID,
		"deleted_at": time.Now(),
		"note":       "Data retained for audit trail. Will be anonymized after retention period.",
	})
}

func (app *App) anonymizeUser(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Params("userId")
	anonymizedBy := c.Get("X-User-ID", "system")

	err := app.gdprService.AnonymizeUser(c.Context(), tenantID, userID, anonymizedBy, c.IP(), c.Get("User-Agent"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to anonymize user: %v", err))
	}

	return c.JSON(fiber.Map{
		"message":       "User anonymized successfully",
		"user_id":       userID,
		"anonymized_at": time.Now(),
		"note":          "Personal data removed. Record retained for legal compliance.",
	})
}

// Document handlers

func (app *App) createDocument(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	createdBy := c.Get("X-User-ID", "system")

	var req struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		OwnerID  string `json:"owner_id"`
		IsPublic bool   `json:"is_public"`
	}

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	document := models.Document{
		TenantID: tenantID,
		Title:    req.Title,
		Content:  req.Content,
		OwnerID:  req.OwnerID,
		IsPublic: req.IsPublic,
		GDPRMetadata: models.GDPRMetadata{
			CreatedBy: &createdBy,
		},
	}

	documents, err := builder.Insert[models.Document](app.db).
		Values(document).
		Returning("*").
		ExecReturning(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to create document: %v", err))
	}

	// Log creation
	_ = app.gdprService.LogAudit(c.Context(), tenantID, createdBy, "CREATE", "documents", documents[0].ID, c.IP(), c.Get("User-Agent"), nil)

	return c.Status(fiber.StatusCreated).JSON(documents[0])
}

func (app *App) listDocuments(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	userID := c.Get("X-User-ID", "system")

	tenantDB := database.NewTenantDB(app.db, tenantID, userID)

	documents, err := database.SelectActive[models.Document](tenantDB).
		OrderByDesc("created_at").
		All(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to list documents: %v", err))
	}

	return c.JSON(documents)
}

func (app *App) getDocument(c *fiber.Ctx) error {
	tenantID := c.Params("tenantId")
	documentID := c.Params("documentId")
	requestedBy := c.Get("X-User-ID", "system")

	documents, err := builder.Select[models.Document](app.db).
		Where(builder.Eq("id", documentID)).
		And(builder.Eq("tenant_id", tenantID)).
		And(builder.IsNull("deleted_at")).
		All(c.Context())

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to get document: %v", err))
	}

	if len(documents) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "Document not found")
	}

	// Log access
	_ = app.gdprService.LogAudit(c.Context(), tenantID, requestedBy, "READ", "documents", documentID, c.IP(), c.Get("User-Agent"), nil)

	return c.JSON(documents[0])
}

// Custom error handler
func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"error":   err.Error(),
		"status":  code,
		"path":    c.Path(),
		"method":  c.Method(),
	})
}
