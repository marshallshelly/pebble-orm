package main

import (
	"context"
	"log"

	"github.com/marshallshelly/pebble-orm/examples/generated_columns/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/generated_columns/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
)

func main() {
	ctx := context.Background()

	// Initialize database connection
	qb, cleanup, err := database.Initialize(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer cleanup()

	log.Println("=== PostgreSQL Generated Columns Example ===\n")

	// Example 1: Full Name Generation
	log.Println("--- Example 1: Full Name (Concatenation) ---")
	people := []models.Person{
		{FirstName: "John", LastName: "Doe"},
		{FirstName: "Jane", LastName: "Smith"},
		{FirstName: "Bob", LastName: "Johnson"},
	}

	insertedPeople, err := builder.Insert[models.Person](qb).
		Values(people...).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert failed: %v", err)
	} else {
		log.Println("Inserted people with auto-generated full names:")
		for _, person := range insertedPeople {
			log.Printf("  - %s (from: %s %s)", person.FullName, person.FirstName, person.LastName)
		}
	}

	// Example 2: Unit Conversions
	log.Println("\n--- Example 2: Unit Conversions (Height & Weight) ---")
	measurements := []models.Measurement{
		{Name: "Person A", HeightCm: 180.0, WeightKg: 75.0},
		{Name: "Person B", HeightCm: 165.5, WeightKg: 62.5},
		{Name: "Person C", HeightCm: 175.0, WeightKg: 80.0},
	}

	insertedMeasurements, err := builder.Insert[models.Measurement](qb).
		Values(measurements...).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert failed: %v", err)
	} else {
		log.Println("Inserted measurements with auto-converted units:")
		for _, m := range insertedMeasurements {
			log.Printf("  - %s:", m.Name)
			log.Printf("      Height: %.2f cm = %.2f inches", m.HeightCm, m.HeightIn)
			log.Printf("      Weight: %.2f kg = %.2f lbs", m.WeightKg, m.WeightLbs)
		}
	}

	// Example 3: Complex Calculations (Net Price)
	log.Println("\n--- Example 3: Complex Calculations (Net Price) ---")
	products := []models.Product{
		{Name: "Widget A", ListPrice: 100.00, Tax: 10.00, Discount: 5.00},  // 10% tax, 5% discount
		{Name: "Widget B", ListPrice: 50.00, Tax: 8.00, Discount: 0.00},    // 8% tax, no discount
		{Name: "Widget C", ListPrice: 120.00, Tax: 12.50, Discount: 10.00}, // 12.5% tax, 10% discount
	}

	insertedProducts, err := builder.Insert[models.Product](qb).
		Values(products...).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert failed: %v", err)
	} else {
		log.Println("Inserted products with auto-calculated net prices:")
		for _, p := range insertedProducts {
			log.Printf("  - %s:", p.Name)
			log.Printf("      List Price: $%.2f", p.ListPrice)
			log.Printf("      Tax: %.2f%%", p.Tax)
			log.Printf("      Discount: %.2f%%", p.Discount)
			log.Printf("      Net Price: $%.2f (auto-calculated)", p.NetPrice)
		}
	}

	// Example 4: Query with Generated Columns
	log.Println("\n--- Example 4: Query Using Generated Columns ---")
	expensiveProducts, err := builder.Select[models.Product](qb).
		Where(builder.Gt(builder.Col[models.Product]("NetPrice"), 100.00)).
		OrderByDesc(builder.Col[models.Product]("NetPrice")).
		All(ctx)
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		log.Printf("Found %d products with net price > $100:", len(expensiveProducts))
		for _, p := range expensiveProducts {
			log.Printf("  - %s: $%.2f", p.Name, p.NetPrice)
		}
	}

	// Example 5: Update Source Columns (Generated Columns Auto-Update)
	log.Println("\n--- Example 5: Update Source Columns ---")
	if len(insertedPeople) > 0 {
		personID := insertedPeople[0].ID
		log.Printf("Updating person ID %d...", personID)

		_, err := builder.Update[models.Person](qb).
			Set(builder.Col[models.Person]("FirstName"), "Johnny").
			Set(builder.Col[models.Person]("LastName"), "Doe Jr.").
			Where(builder.Eq(builder.Col[models.Person]("ID"), personID)).
			Exec(ctx)
		if err != nil {
			log.Printf("Update failed: %v", err)
		} else {
			// Fetch updated person
			updatedPerson, err := builder.Select[models.Person](qb).
				Where(builder.Eq(builder.Col[models.Person]("ID"), personID)).
				One(ctx)
			if err != nil {
				log.Printf("Fetch failed: %v", err)
			} else {
				log.Printf("Updated person:")
				log.Printf("  - Full Name: %s (auto-updated from: %s %s)",
					updatedPerson.FullName, updatedPerson.FirstName, updatedPerson.LastName)
			}
		}
	}

	log.Println("\n✅ All generated columns examples completed!")
	log.Println("\nKey Takeaways:")
	log.Println("  - Generated columns are automatically computed by the database")
	log.Println("  - You only INSERT/UPDATE the source columns")
	log.Println("  - The database keeps generated values in sync automatically")
	log.Println("  - Generated columns can be queried and indexed like regular columns")
	log.Println("  - Perfect for: concatenation, unit conversion, calculations")
}
