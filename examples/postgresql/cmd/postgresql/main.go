package main

import (
	"context"
	"fmt"
	"log"

	"github.com/marshallshelly/pebble-orm/examples/postgresql/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/postgresql/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func main() {
	ctx := context.Background()

	log.Println("=== PostgreSQL Features Example ===\n")

	// Connect to database
	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("✅ Connected to database\n")

	// Create query builder
	qb := builder.New(db)

	// Example 1: JSONB Operations
	fmt.Println("--- Example 1: JSONB (JSON Binary) ---")
	exampleJSONB(ctx, qb)

	// Example 2: Array Operations
	fmt.Println("\n--- Example 2: PostgreSQL Arrays ---")
	exampleArrays(ctx, qb)

	// Example 3: Full-Text Search
	fmt.Println("\n--- Example 3: Full-Text Search ---")
	exampleFullTextSearch(ctx, qb)

	// Example 4: Geometric Types
	fmt.Println("\n--- Example 4: Geometric Types ---")
	exampleGeometric(ctx, qb)

	log.Println("\n✅ All PostgreSQL feature examples completed!")
	log.Println("\nKey Takeaways:")
	log.Println("  - JSONB: Store and query JSON data efficiently")
	log.Println("  - Arrays: Native PostgreSQL array support")
	log.Println("  - Full-Text Search: tsvector and tsquery")
	log.Println("  - Geometric Types: point, line, polygon")
}

func exampleJSONB(ctx context.Context, qb *builder.DB) {
	// Create document with JSONB metadata
	doc := models.Document{
		Title:   "PostgreSQL Guide",
		Content: "Complete guide to PostgreSQL features",
		Metadata: schema.JSONB{
			"author": "John Doe",
			"tags":   []string{"database", "postgresql"},
			"views":  1000,
		},
	}

	result, err := builder.Insert[models.Document](qb).
		Values(doc).
		Returning("*").
		ExecReturning(ctx)

	if err != nil {
		log.Printf("Insert failed: %v\n", err)
		return
	}

	fmt.Println("✅ Created document with JSONB metadata")
	if len(result) > 0 {
		inserted := result[0]
		fmt.Printf("  Title: %s\n", inserted.Title)
		fmt.Printf("  Metadata: %v\n", inserted.Metadata)
	}

	// Query documents by JSONB field
	fmt.Println("\n  Querying by JSONB field...")
	// Note: Use raw SQL for JSONB queries
	fmt.Println("  Use: WHERE metadata->>'author' = 'John Doe'")
}

func exampleArrays(ctx context.Context, qb *builder.DB) {
	// Create document with tags array
	doc := models.Document{
		Title:   "Array Example",
		Content: "Demonstrating PostgreSQL arrays",
		Tags:    []string{"example", "arrays", "postgresql"},
	}

	_, err := builder.Insert[models.Document](qb).Values(doc).Exec(ctx)
	if err != nil {
		log.Printf("Insert failed: %v\n", err)
		return
	}

	fmt.Println("✅ Created document with tags array")
	fmt.Printf("  Tags: %v\n", doc.Tags)

	// Create product with prices array
	product := models.Product{
		Name:   "Widget",
		Prices: []int{100, 150, 200},
		Active: true,
	}

	_, err = builder.Insert[models.Product](qb).Values(product).Exec(ctx)
	if err != nil {
		log.Printf("Insert failed: %v\n", err)
		return
	}

	fmt.Println("✅ Created product with prices array")
	fmt.Printf("  Prices: %v\n", product.Prices)
}

func exampleFullTextSearch(ctx context.Context, qb *builder.DB) {
	fmt.Println("✅ Full-text search uses tsvector and tsquery")
	fmt.Println("  Example SQL:")
	fmt.Println("  UPDATE documents SET search_vec = to_tsvector(content)")
	fmt.Println("  SELECT * FROM documents WHERE search_vec @@ to_tsquery('postgresql')")
	fmt.Println("  Note: Use raw SQL or builder.Raw() for FTS queries")
}

func exampleGeometric(ctx context.Context, qb *builder.DB) {
	location := models.Location{
		Name:   "San Francisco",
		Coords: "(37.7749,-122.4194)", // Point format
	}

	_, err := builder.Insert[models.Location](qb).Values(location).Exec(ctx)
	if err != nil {
		log.Printf("Insert failed: %v\n", err)
		return
	}

	fmt.Println("✅ Created location with geometric point")
	fmt.Printf("  Name: %s\n", location.Name)
	fmt.Printf("  Coords: %s\n", location.Coords)
	fmt.Println("  Note: Use geometric operators like <->, &&, @> for queries")
}
