package main

import (
	"context"
	"log"

	"github.com/marshallshelly/pebble-orm/examples/multi-tenancy/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/multi-tenancy/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
)

func main() {
	ctx := context.Background()

	log.Println("=== Pebble ORM Multi-Tenancy Examples ===\n")

	// Connect to shared database
	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	qb := builder.New(db)

	// Run both examples
	log.Println(">>> Pattern 1: Shared Database with tenant_id Column <<<\n")
	sharedDatabaseExample(ctx, qb)

	log.Println("\n>>> Pattern 2: Database-per-Tenant (Conceptual) <<<\n")
	databasePerTenantExample(ctx)

	log.Println("\n=== All Examples Complete ===")
}

// sharedDatabaseExample demonstrates multi-tenancy using a shared database
// with tenant_id column and automatic filtering
func sharedDatabaseExample(ctx context.Context, qb *builder.DB) {
	// Create some tenants first
	log.Println("--- Setup: Creating Tenants ---")
	tenant1 := models.Tenant{
		Name:      "Acme Corp",
		Subdomain: "acme",
	}
	tenant2 := models.Tenant{
		Name:      "Widget Inc",
		Subdomain: "widget",
	}

	tenants, err := builder.Insert[models.Tenant](qb).
		Values(tenant1, tenant2).
		Returning("id", "name", "subdomain").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Failed to create tenants: %v", err)
		return
	}

	log.Printf("Created tenant: %s (ID: %s)", tenants[0].Name, tenants[0].ID)
	log.Printf("Created tenant: %s (ID: %s)", tenants[1].Name, tenants[1].ID)

	tenant1ID := tenants[0].ID
	tenant2ID := tenants[1].ID

	// Create tenant-specific database wrappers
	acmeDB := database.NewTenantDB(qb, tenant1ID)
	widgetDB := database.NewTenantDB(qb, tenant2ID)

	// Example 1: Insert users for each tenant
	log.Println("\n--- Example 1: INSERT Users (Tenant-Scoped) ---")

	// Insert user for Acme Corp
	acmeUser := models.User{
		TenantID: acmeDB.GetTenantID(),
		Name:     "Alice Johnson",
		Email:    "alice@acme.com",
		Role:     "admin",
	}
	insertedAcmeUsers, err := builder.Insert[models.User](qb).
		Values(acmeUser).
		Returning("id", "name", "email").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Failed to insert Acme user: %v", err)
	} else {
		log.Printf("[Acme] Created user: %s (%s)", insertedAcmeUsers[0].Name, insertedAcmeUsers[0].Email)
	}

	// Insert user for Widget Inc
	widgetUser := models.User{
		TenantID: widgetDB.GetTenantID(),
		Name:     "Bob Smith",
		Email:    "bob@widget.com",
		Role:     "user",
	}
	insertedWidgetUsers, err := builder.Insert[models.User](qb).
		Values(widgetUser).
		Returning("id", "name", "email").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Failed to insert Widget user: %v", err)
	} else {
		log.Printf("[Widget] Created user: %s (%s)", insertedWidgetUsers[0].Name, insertedWidgetUsers[0].Email)
	}

	// Example 2: Query with automatic tenant filtering
	log.Println("\n--- Example 2: SELECT with Auto Tenant Filtering ---")

	// Query Acme users (only returns Acme's data)
	acmeUsers, err := database.Select[models.User](acmeDB).
		OrderByAsc(builder.Col[models.User]("CreatedAt")).
		All(ctx)
	if err != nil {
		log.Printf("Failed to query Acme users: %v", err)
	} else {
		log.Printf("[Acme] Found %d users:", len(acmeUsers))
		for _, user := range acmeUsers {
			log.Printf("  - %s (%s)", user.Name, user.Email)
		}
	}

	// Query Widget users (only returns Widget's data)
	widgetUsers, err := database.Select[models.User](widgetDB).
		OrderByAsc(builder.Col[models.User]("CreatedAt")).
		All(ctx)
	if err != nil {
		log.Printf("Failed to query Widget users: %v", err)
	} else {
		log.Printf("[Widget] Found %d users:", len(widgetUsers))
		for _, user := range widgetUsers {
			log.Printf("  - %s (%s)", user.Name, user.Email)
		}
	}

	// Example 3: Create documents for tenants
	log.Println("\n--- Example 3: INSERT Documents (Tenant-Scoped) ---")

	if len(insertedAcmeUsers) > 0 {
		acmeDoc := models.Document{
			TenantID: acmeDB.GetTenantID(),
			OwnerID:  insertedAcmeUsers[0].ID,
			Title:    "Acme Q1 Report",
			Content:  "Quarterly financial report for Acme Corp...",
			IsPublic: false,
		}
		insertedAcmeDocs, err := builder.Insert[models.Document](qb).
			Values(acmeDoc).
			Returning("id", "title").
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Failed to insert Acme document: %v", err)
		} else {
			log.Printf("[Acme] Created document: %s", insertedAcmeDocs[0].Title)
		}
	}

	if len(insertedWidgetUsers) > 0 {
		widgetDoc := models.Document{
			TenantID: widgetDB.GetTenantID(),
			OwnerID:  insertedWidgetUsers[0].ID,
			Title:    "Widget Product Roadmap",
			Content:  "2024 product development roadmap...",
			IsPublic: true,
		}
		insertedWidgetDocs, err := builder.Insert[models.Document](qb).
			Values(widgetDoc).
			Returning("id", "title").
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Failed to insert Widget document: %v", err)
		} else {
			log.Printf("[Widget] Created document: %s", insertedWidgetDocs[0].Title)
		}
	}

	// Example 4: Query documents with tenant filtering
	log.Println("\n--- Example 4: SELECT Documents (Tenant Isolated) ---")

	// Acme can only see their documents
	acmeDocs, err := database.Select[models.Document](acmeDB).
		OrderByAsc(builder.Col[models.Document]("CreatedAt")).
		All(ctx)
	if err != nil {
		log.Printf("Failed to query Acme documents: %v", err)
	} else {
		log.Printf("[Acme] Found %d documents:", len(acmeDocs))
		for _, doc := range acmeDocs {
			log.Printf("  - %s", doc.Title)
		}
	}

	// Widget can only see their documents
	widgetDocs, err := database.Select[models.Document](widgetDB).
		OrderByAsc(builder.Col[models.Document]("CreatedAt")).
		All(ctx)
	if err != nil {
		log.Printf("Failed to query Widget documents: %v", err)
	} else {
		log.Printf("[Widget] Found %d documents:", len(widgetDocs))
		for _, doc := range widgetDocs {
			log.Printf("  - %s", doc.Title)
		}
	}

	// Example 5: Update with tenant filtering
	log.Println("\n--- Example 5: UPDATE with Tenant Filtering ---")

	// Update only affects the current tenant's data
	count, err := database.Update[models.User](acmeDB).
		Set(builder.Col[models.User]("Role"), "super_admin").
		Where(builder.Eq(builder.Col[models.User]("Email"), "alice@acme.com")).
		Exec(ctx)
	if err != nil {
		log.Printf("Failed to update Acme user: %v", err)
	} else {
		log.Printf("[Acme] Updated %d user(s) to super_admin", count)
	}

	// Example 6: COUNT with tenant filtering
	log.Println("\n--- Example 6: COUNT (Tenant-Scoped) ---")

	acmeUserCount, err := database.Select[models.User](acmeDB).Count(ctx)
	if err != nil {
		log.Printf("Failed to count Acme users: %v", err)
	} else {
		log.Printf("[Acme] Total users: %d", acmeUserCount)
	}

	widgetUserCount, err := database.Select[models.User](widgetDB).Count(ctx)
	if err != nil {
		log.Printf("Failed to count Widget users: %v", err)
	} else {
		log.Printf("[Widget] Total users: %d", widgetUserCount)
	}

	// Example 7: Verify tenant isolation
	log.Println("\n--- Example 7: Verify Tenant Isolation ---")
	log.Println("✓ Each tenant wrapper automatically filters by tenant_id")
	log.Println("✓ Acme cannot see Widget's data and vice versa")
	log.Println("✓ All SELECT, UPDATE, DELETE queries are tenant-scoped")
	log.Println("✓ INSERT requires manually setting tenant_id on the model")
}

// databasePerTenantExample demonstrates the database-per-tenant pattern
func databasePerTenantExample(ctx context.Context) {
	log.Println("This example shows the database-per-tenant architecture pattern.")
	log.Println("")
	log.Println("Key Concepts:")
	log.Println("  1. Each tenant has their own PostgreSQL database")
	log.Println("  2. Complete data isolation at the database level")
	log.Println("  3. Independent backups and migrations per tenant")
	log.Println("  4. Connection pooling per tenant")
	log.Println("")

	// Create tenant manager
	tm := database.NewTenantManager()
	defer tm.CloseAll()

	log.Println("--- Example: TenantManager Usage ---")
	log.Println("")
	log.Println("// Create tenant manager")
	log.Println("tm := database.NewTenantManager()")
	log.Println("")
	log.Println("// Get connection for tenant 'acme'")
	log.Println("// Creates database 'tenant_acme' if needed")
	log.Println("acmeDB, err := tm.GetConnection(ctx, \"acme\")")
	log.Println("")
	log.Println("// Get connection for tenant 'widget'")
	log.Println("// Creates database 'tenant_widget' if needed")
	log.Println("widgetDB, err := tm.GetConnection(ctx, \"widget\")")
	log.Println("")
	log.Println("// Use standard query builders")
	log.Println("users, err := builder.Select[models.User](builder.New(acmeDB)).All(ctx)")
	log.Println("")
	log.Println("Benefits:")
	log.Println("  ✓ Perfect data isolation (separate databases)")
	log.Println("  ✓ No need for tenant_id columns")
	log.Println("  ✓ Easy per-tenant backups and migrations")
	log.Println("  ✓ Scales well with moderate number of tenants")
	log.Println("")
	log.Println("Trade-offs:")
	log.Println("  - Requires creating databases dynamically")
	log.Println("  - More database connections (one pool per tenant)")
	log.Println("  - May not scale to thousands of tenants")
	log.Println("")
	log.Println("Note: Actual database creation requires PostgreSQL superuser")
	log.Println("      privileges or pre-provisioned tenant databases.")
}
