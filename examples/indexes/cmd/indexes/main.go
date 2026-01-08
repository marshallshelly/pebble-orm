package main

import (
	"context"
	"log"
	"time"

	"github.com/marshallshelly/pebble-orm/examples/indexes/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/indexes/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func main() {
	ctx := context.Background()

	// Initialize database connection
	qb, cleanup, err := database.Initialize(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer cleanup()

	log.Println("=== Pebble ORM Index Examples ===\n")

	// Example 1: Simple Column-Level Indexes
	log.Println("--- Example 1: Simple Column-Level Indexes ---")
	product := models.Product{
		Name:        "PostgreSQL Database Book",
		SKU:         "PG-BOOK-001",
		Price:       49.99,
		Category:    "Books",
		InStock:     true,
		Tags:        []string{"database", "postgresql", "technical"},
		Description: "Comprehensive guide to PostgreSQL",
	}
	insertedProducts, err := builder.Insert[models.Product](qb).
		Values(product).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert product failed: %v", err)
	} else {
		log.Printf("✓ Inserted product: %s", insertedProducts[0].Name)
	}

	// Query using indexed columns (name, sku, price, category, created_at)
	products, err := builder.Select[models.Product](qb).
		Where(builder.Eq(builder.Col[models.Product]("Category"), "Books")).
		And(builder.Gte(builder.Col[models.Product]("Price"), 30.0)).
		OrderByDesc(builder.Col[models.Product]("CreatedAt")).
		Limit(10).
		All(ctx)
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		log.Printf("✓ Found %d products (uses idx_products_category and idx_products_price)\n", len(products))
	}

	// Example 2: GIN Index for Array Searches
	log.Println("--- Example 2: GIN Index for Array Searches ---")
	// Find products by tags using GIN index (idx_product_tags)
	taggedProducts, err := builder.Select[models.Product](qb).
		Where(builder.ArrayContains(builder.Col[models.Product]("Tags"), []string{"postgresql"})).
		All(ctx)
	if err != nil {
		log.Printf("Array query failed: %v", err)
	} else {
		log.Printf("✓ Found %d products with 'postgresql' tag (uses idx_product_tags GIN index)\n", len(taggedProducts))
	}

	// Example 3: Expression Indexes and Partial Indexes
	log.Println("--- Example 3: Expression Indexes and Partial Indexes ---")
	user := models.User{
		Email:            "alice.smith@example.com",
		FirstName:        "Alice",
		LastName:         "Smith",
		SubscriptionTier: "premium",
	}
	insertedUsers, err := builder.Insert[models.User](qb).
		Values(user).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert user failed: %v", err)
	} else {
		log.Printf("✓ Inserted user: %s %s", insertedUsers[0].FirstName, insertedUsers[0].LastName)
	}

	// Query using expression index on lower(email) - idx_email_lower
	// Query using partial index on active users - idx_active_users (WHERE deleted_at IS NULL)
	// Query using partial index on premium users - idx_premium_users (WHERE subscription_tier = 'premium')
	activeUsers, err := builder.Select[models.User](qb).
		Where(builder.IsNull(builder.Col[models.User]("DeletedAt"))).
		And(builder.Eq(builder.Col[models.User]("SubscriptionTier"), "premium")).
		All(ctx)
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		log.Printf("✓ Found %d active premium users (uses idx_premium_users partial index)\n", len(activeUsers))
	}

	// Example 4: Covering Indexes (INCLUDE columns)
	log.Println("--- Example 4: Covering Indexes (INCLUDE columns) ---")
	if len(insertedUsers) > 0 {
		order := models.Order{
			CustomerID:   insertedUsers[0].ID,
			Status:       "pending",
			TotalAmount:  149.99,
			ShippingAddr: "123 Main St, New York, NY 10001",
		}
		insertedOrders, err := builder.Insert[models.Order](qb).
			Values(order).
			Returning("*").
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Insert order failed: %v", err)
		} else {
			log.Printf("✓ Inserted order: ID=%d, Amount=$%.2f", insertedOrders[0].ID, insertedOrders[0].TotalAmount)
		}

		// Query using covering index idx_orders_customer_status
		// This query can be satisfied entirely from the index (index-only scan)
		orders, err := builder.Select[models.Order](qb).
			Columns(
				builder.Col[models.Order]("CustomerID"),
				builder.Col[models.Order]("Status"),
				builder.Col[models.Order]("TotalAmount"),
				builder.Col[models.Order]("CreatedAt"),
			).
			Where(builder.Eq(builder.Col[models.Order]("CustomerID"), insertedUsers[0].ID)).
			And(builder.Eq(builder.Col[models.Order]("Status"), "pending")).
			All(ctx)
		if err != nil {
			log.Printf("Query failed: %v", err)
		} else {
			log.Printf("✓ Found %d pending orders (uses idx_orders_customer_status covering index - index-only scan!)\n", len(orders))
		}
	}

	// Example 5: Multicolumn Indexes with Mixed Ordering
	log.Println("--- Example 5: Multicolumn Indexes with Mixed Ordering ---")
	if len(insertedUsers) > 0 {
		event := models.Event{
			TenantID:  1,
			UserID:    insertedUsers[0].ID,
			EventType: "page_view",
			Data: schema.JSONB{
				"page":     "/products",
				"duration": 45,
			},
			IPAddress: "192.168.1.100",
		}
		insertedEvents, err := builder.Insert[models.Event](qb).
			Values(event).
			Returning("*").
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Insert event failed: %v", err)
		} else {
			log.Printf("✓ Inserted event: %s for tenant %d", insertedEvents[0].EventType, insertedEvents[0].TenantID)
		}

		// Query using idx_events_tenant_created (tenant_id, created_at DESC NULLS LAST)
		// Perfect for paginated queries of recent events by tenant
		events, err := builder.Select[models.Event](qb).
			Where(builder.Eq(builder.Col[models.Event]("TenantID"), int64(1))).
			OrderByDesc(builder.Col[models.Event]("CreatedAt")).
			Limit(20).
			All(ctx)
		if err != nil {
			log.Printf("Query failed: %v", err)
		} else {
			log.Printf("✓ Found %d recent events for tenant (uses idx_events_tenant_created multicolumn index)\n", len(events))
		}

		// Query using idx_events_data GIN index for JSONB
		eventsWithData, err := builder.Select[models.Event](qb).
			Where(builder.JSONBContains(builder.Col[models.Event]("Data"), `{"page": "/products"}`)).
			All(ctx)
		if err != nil {
			log.Printf("JSONB query failed: %v", err)
		} else {
			log.Printf("✓ Found %d events on /products page (uses idx_events_data GIN index)\n", len(eventsWithData))
		}
	}

	// Example 6: Operator Classes for Pattern Matching
	log.Println("--- Example 6: Operator Classes for Pattern Matching ---")
	searchTerm := models.SearchTerm{
		Term:        "postgresql database",
		Description: "Advanced database management system",
		SearchCount: 42,
	}
	insertedSearchTerms, err := builder.Insert[models.SearchTerm](qb).
		Values(searchTerm).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert search term failed: %v", err)
	} else {
		log.Printf("✓ Inserted search term: %s", insertedSearchTerms[0].Term)
	}

	// Query using idx_search_term_pattern with varchar_pattern_ops
	// This index is optimized for LIKE queries with leading wildcards
	terms, err := builder.Select[models.SearchTerm](qb).
		Where(builder.Like(builder.Col[models.SearchTerm]("Term"), "postgres%")).
		OrderByDesc(builder.Col[models.SearchTerm]("SearchCount")).
		All(ctx)
	if err != nil {
		log.Printf("Pattern query failed: %v", err)
	} else {
		log.Printf("✓ Found %d terms matching 'postgres%%' (uses idx_search_term_pattern with varchar_pattern_ops)\n", len(terms))
	}

	// Example 7: DESC Ordering with NULLS LAST
	log.Println("--- Example 7: DESC Ordering with NULLS LAST ---")
	task := models.Task{
		Title:    "Implement PostgreSQL indexes",
		Priority: intPtr(1),
		DueDate:  timePtr(time.Now().Add(7 * 24 * time.Hour)),
	}
	insertedTasks, err := builder.Insert[models.Task](qb).
		Values(task).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert task failed: %v", err)
	} else {
		log.Printf("✓ Inserted task: %s", insertedTasks[0].Title)
	}

	// Query using idx_tasks_priority (priority DESC NULLS LAST, due_date ASC NULLS FIRST)
	// This ensures tasks with priority come first, then unprioritized tasks
	tasks, err := builder.Select[models.Task](qb).
		Where(builder.IsNull(builder.Col[models.Task]("CompletedAt"))).
		OrderByDesc(builder.Col[models.Task]("Priority")).
		OrderBy(builder.Col[models.Task]("DueDate")).
		All(ctx)
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		log.Printf("✓ Found %d incomplete tasks ordered by priority (uses idx_tasks_priority)\n", len(tasks))
	}

	// Example 8: BRIN Index for Time-Series Data
	log.Println("--- Example 8: BRIN Index for Time-Series Data ---")
	reading := models.SensorReading{
		DeviceID:   "sensor-001",
		SensorType: "temperature",
		Value:      72.5,
		Unit:       "fahrenheit",
	}
	insertedReadings, err := builder.Insert[models.SensorReading](qb).
		Values(reading).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert sensor reading failed: %v", err)
	} else {
		log.Printf("✓ Inserted sensor reading: %.2f %s", insertedReadings[0].Value, insertedReadings[0].Unit)
	}

	// Query using idx_sensor_timestamp BRIN index
	// BRIN indexes are extremely space-efficient for naturally-ordered data
	recentReadings, err := builder.Select[models.SensorReading](qb).
		Where(builder.Gte(builder.Col[models.SensorReading]("RecordedAt"), time.Now().Add(-1*time.Hour))).
		OrderByDesc(builder.Col[models.SensorReading]("RecordedAt")).
		Limit(100).
		All(ctx)
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		log.Printf("✓ Found %d readings in last hour (uses idx_sensor_timestamp BRIN index)\n", len(recentReadings))
	}

	// Example 9: Hash Index for Equality-Only Queries
	log.Println("--- Example 9: Hash Index for Equality-Only Queries ---")
	apiKey := models.APIKey{
		KeyHash: "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3", // SHA-256 of "123"
		UserID:  insertedUsers[0].ID,
		Name:    "Production API Key",
	}
	insertedAPIKeys, err := builder.Insert[models.APIKey](qb).
		Values(apiKey).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert API key failed: %v", err)
	} else {
		log.Printf("✓ Inserted API key: %s", insertedAPIKeys[0].Name)
	}

	// Query using idx_api_key_hash HASH index
	// Hash indexes are faster for equality checks but cannot be used for range queries
	keys, err := builder.Select[models.APIKey](qb).
		Where(builder.Eq(builder.Col[models.APIKey]("KeyHash"), apiKey.KeyHash)).
		All(ctx)
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		log.Printf("✓ Found API key by hash (uses idx_api_key_hash HASH index)\n")
		if len(keys) > 0 {
			log.Printf("  Key: %s", keys[0].Name)
		}
	}

	// Example 10: Complex Multi-Feature Index
	log.Println("--- Example 10: Complex Multi-Feature Index ---")
	if len(insertedUsers) > 0 {
		document := models.Document{
			OwnerID: insertedUsers[0].ID,
			Title:   "PostgreSQL Index Optimization Guide",
			Content: "This guide covers all aspects of PostgreSQL index optimization including btree, gin, gist, and brin indexes.",
			Status:  "published",
			Version: 1,
			Tags:    []string{"postgresql", "performance", "indexes"},
		}
		insertedDocs, err := builder.Insert[models.Document](qb).
			Values(document).
			Returning("*").
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Insert document failed: %v", err)
		} else {
			log.Printf("✓ Inserted document: %s", insertedDocs[0].Title)
		}

		// Query using idx_documents_complex
		// (owner_id, status, updated_at DESC NULLS LAST) INCLUDE (title, version) WHERE deleted_at IS NULL
		// This is a partial covering index perfect for user dashboards
		docs, err := builder.Select[models.Document](qb).
			Columns(
				builder.Col[models.Document]("OwnerID"),
				builder.Col[models.Document]("Status"),
				builder.Col[models.Document]("Title"),
				builder.Col[models.Document]("Version"),
				builder.Col[models.Document]("UpdatedAt"),
			).
			Where(builder.Eq(builder.Col[models.Document]("OwnerID"), insertedUsers[0].ID)).
			And(builder.Eq(builder.Col[models.Document]("Status"), "published")).
			And(builder.IsNull(builder.Col[models.Document]("DeletedAt"))).
			OrderByDesc(builder.Col[models.Document]("UpdatedAt")).
			All(ctx)
		if err != nil {
			log.Printf("Query failed: %v", err)
		} else {
			log.Printf("✓ Found %d published documents (uses idx_documents_complex partial covering index)\n", len(docs))
		}

		// Query using idx_documents_tags GIN index
		taggedDocs, err := builder.Select[models.Document](qb).
			Where(builder.ArrayContains(builder.Col[models.Document]("Tags"), []string{"postgresql"})).
			All(ctx)
		if err != nil {
			log.Printf("Array query failed: %v", err)
		} else {
			log.Printf("✓ Found %d documents tagged with 'postgresql' (uses idx_documents_tags GIN index)\n", len(taggedDocs))
		}
	}

	// Example 11: Collations for Locale-Specific Sorting
	log.Println("--- Example 11: Collations for Locale-Specific Sorting ---")
	intlProduct := models.InternationalProduct{
		Name:        "Café Parisien",
		NameLocal:   "Café Parisien",
		Price:       12.99,
		Locale:      "fr_FR",
		CountryCode: "FR",
	}
	insertedIntlProducts, err := builder.Insert[models.InternationalProduct](qb).
		Values(intlProduct).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert international product failed: %v", err)
	} else {
		log.Printf("✓ Inserted international product: %s (%s)", insertedIntlProducts[0].Name, insertedIntlProducts[0].Locale)
	}

	// Query using idx_intl_name_en (name COLLATE "en_US")
	// This index provides locale-specific sorting
	intlProducts, err := builder.Select[models.InternationalProduct](qb).
		Where(builder.Eq(builder.Col[models.InternationalProduct]("CountryCode"), "FR")).
		OrderBy(builder.Col[models.InternationalProduct]("Name")).
		All(ctx)
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		log.Printf("✓ Found %d French products sorted by name (uses idx_intl_name_en with en_US collation)\n", len(intlProducts))
	}

	// Example 12: CONCURRENTLY for Production
	log.Println("--- Example 12: CONCURRENTLY for Production ---")
	if len(insertedUsers) > 0 {
		analyticsEvent := models.AnalyticsEvent{
			UserID:         &insertedUsers[0].ID,
			SessionID:      "session-12345",
			EventType:      "page_view",
			EventTimestamp: time.Now(),
			Processed:      false,
			Properties: schema.JSONB{
				"page":     "/dashboard",
				"referrer": "https://example.com",
			},
		}
		insertedAnalytics, err := builder.Insert[models.AnalyticsEvent](qb).
			Values(analyticsEvent).
			Returning("*").
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Insert analytics event failed: %v", err)
		} else {
			log.Printf("✓ Inserted analytics event: %s", insertedAnalytics[0].EventType)
		}

		// The CONCURRENTLY indexes would be created during migration without blocking writes
		// idx_analytics_timestamp ON (event_timestamp DESC) CONCURRENTLY
		// idx_analytics_user_session ON (user_id, session_id) CONCURRENTLY
		// idx_analytics_event_type ON (event_type) CONCURRENTLY WHERE processed = false
		// idx_analytics_props ON (properties) USING gin CONCURRENTLY

		analytics, err := builder.Select[models.AnalyticsEvent](qb).
			Where(builder.Eq(builder.Col[models.AnalyticsEvent]("Processed"), false)).
			OrderByDesc(builder.Col[models.AnalyticsEvent]("EventTimestamp")).
			Limit(50).
			All(ctx)
		if err != nil {
			log.Printf("Query failed: %v", err)
		} else {
			log.Printf("✓ Found %d unprocessed analytics events (uses idx_analytics_event_type partial index)\n", len(analytics))
		}
	}

	log.Println("\n=== Index Examples Complete ===")
	log.Println("\nKey Takeaways:")
	log.Println("• Simple column indexes improve basic WHERE and ORDER BY queries")
	log.Println("• GIN indexes enable efficient JSONB and array queries")
	log.Println("• Expression indexes optimize queries on computed values")
	log.Println("• Partial indexes reduce size and improve performance for filtered data")
	log.Println("• Covering indexes (INCLUDE) enable index-only scans")
	log.Println("• Operator classes optimize specific query patterns (LIKE, etc.)")
	log.Println("• Multicolumn indexes with mixed ordering support complex queries")
	log.Println("• BRIN indexes are space-efficient for time-series data")
	log.Println("• Hash indexes are fastest for equality-only lookups")
	log.Println("• CONCURRENTLY creates indexes without blocking writes in production")
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func timePtr(t time.Time) *time.Time {
	return &t
}
