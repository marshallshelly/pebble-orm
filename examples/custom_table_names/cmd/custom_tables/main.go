package main

import (
	"context"
	"log"

	"github.com/marshallshelly/pebble-orm/examples/custom_table_names/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/custom_table_names/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
)

func main() {
	ctx := context.Background()

	log.Println("=== Custom Table Names Example ===\n")

	// Connect to database
	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("✅ Connected to database\n")

	// Display actual table names
	log.Println("--- Table Name Mappings ---")
	tableNames := database.GetTableNames()
	for structName, tableName := range tableNames {
		log.Printf("  %s struct → %s table", structName, tableName)
	}
	log.Println()

	// Create query builder
	qb := builder.New(db)

	// Example 1: Insert user into custom_users_table
	log.Println("--- Example 1: INSERT into custom_users_table ---")
	user := models.User{
		Name:  "John Doe",
		Email: "john@example.com",
	}

	insertedUsers, err := builder.Insert[models.User](qb).
		Values(user).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert failed: %v\n", err)
	} else {
		log.Printf("✅ Inserted user into 'custom_users_table': %+v\n", insertedUsers[0])
	}

	// Example 2: Insert product into products_inventory
	log.Println("\n--- Example 2: INSERT into products_inventory ---")
	product := models.Product{
		Name:  "Laptop",
		Price: 999,
		Stock: 50,
	}

	insertedProducts, err := builder.Insert[models.Product](qb).
		Values(product).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert failed: %v\n", err)
	} else {
		log.Printf("✅ Inserted product into 'products_inventory': %+v\n", insertedProducts[0])
	}

	// Example 3: Insert order into default 'order' table
	log.Println("\n--- Example 3: INSERT into 'order' (default snake_case) ---")
	if len(insertedUsers) > 0 && len(insertedProducts) > 0 {
		order := models.Order{
			UserID:    insertedUsers[0].ID,
			ProductID: insertedProducts[0].ID,
			Quantity:  2,
			Total:     insertedProducts[0].Price * 2,
		}

		insertedOrders, err := builder.Insert[models.Order](qb).
			Values(order).
			Returning("*").
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Insert failed: %v\n", err)
		} else {
			log.Printf("✅ Inserted order into 'order' table: %+v\n", insertedOrders[0])
		}
	}

	// Example 4: Query with custom table names
	log.Println("\n--- Example 4: SELECT from custom tables ---")
	users, err := builder.Select[models.User](qb).All(ctx)
	if err != nil {
		log.Printf("Select failed: %v\n", err)
	} else {
		log.Printf("✅ Found %d users in 'custom_users_table'\n", len(users))
	}

	products, err := builder.Select[models.Product](qb).All(ctx)
	if err != nil {
		log.Printf("Select failed: %v\n", err)
	} else {
		log.Printf("✅ Found %d products in 'products_inventory'\n", len(products))
	}

	orders, err := builder.Select[models.Order](qb).
		Preload("User").
		Preload("Product").
		All(ctx)
	if err != nil {
		log.Printf("Select failed: %v\n", err)
	} else {
		log.Printf("✅ Found %d orders in 'order' table\n", len(orders))
		for _, ord := range orders {
			userName := "Unknown"
			productName := "Unknown"
			if ord.User != nil {
				userName = ord.User.Name
			}
			if ord.Product != nil {
				productName = ord.Product.Name
			}
			log.Printf("  Order #%d: %s bought %d x %s\n", ord.ID, userName, ord.Quantity, productName)
		}
	}

	log.Println("\n✅ Custom table names work perfectly!")
	log.Println("\nKey Takeaway:")
	log.Println("  - Use `// table_name: custom_name` comments to override defaults")
	log.Println("  - Without directive, Pebble uses snake_case conversion")
	log.Println("  - Perfect for legacy databases or custom naming conventions")
}
