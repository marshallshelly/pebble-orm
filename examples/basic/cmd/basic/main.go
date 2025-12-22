package main

import (
	"context"
	"log"

	"github.com/marshallshelly/pebble-orm/examples/basic/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/basic/internal/models"
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

	log.Println("=== Pebble ORM Basic Examples ===")

	// Example 1: INSERT with RETURNING
	log.Println("\n--- Example 1: INSERT with RETURNING ---")
	newUsers := []models.User{
		{Name: "Alice Smith", Email: "alice@example.com", Age: 28},
		{Name: "Bob Johnson", Email: "bob@example.com", Age: 35},
	}
	insertedUsers, err := builder.Insert[models.User](qb).
		Values(newUsers...).
		Returning(
			builder.Col[models.User]("ID"),
			builder.Col[models.User]("Name"),
			builder.Col[models.User]("Email"),
		).
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert failed: %v", err)
	} else {
		log.Printf("Inserted user: %+v", insertedUsers[0])
	}

	// Example 2: SELECT with WHERE
	log.Println("\n--- Example 2: SELECT with WHERE ---")
	users, err := builder.Select[models.User](qb).
		Where(builder.Gte(builder.Col[models.User]("Age"), 18)).
		OrderByDesc(builder.Col[models.User]("CreatedAt")).
		Limit(10).
		All(ctx)
	if err != nil {
		log.Printf("Select failed: %v", err)
	} else {
		log.Printf("Found %d users:", len(users))
		for _, user := range users {
			log.Printf("  - %s (%s)", user.Name, user.Email)
		}
	}

	// Example 3: UPDATE
	log.Println("\n--- Example 3: UPDATE ---")
	count, err := builder.Update[models.User](qb).
		Set(builder.Col[models.User]("Age"), 29).
		Where(builder.Eq(builder.Col[models.User]("Email"), "alice@example.com")).
		Exec(ctx)
	if err != nil {
		log.Printf("Update failed: %v", err)
	} else {
		log.Printf("Updated %d rows", count)
	}

	// Example 4: INSERT a post
	log.Println("\n--- Example 4: INSERT Post ---")
	if len(insertedUsers) > 0 {
		newPost := models.Post{
			Title:     "Getting Started with Pebble ORM",
			Content:   "Pebble ORM is a type-safe PostgreSQL ORM for Go...",
			AuthorID:  insertedUsers[0].ID,
			Published: true,
		}
		insertedPosts, err := builder.Insert[models.Post](qb).
			Values(newPost).
			Returning(
				builder.Col[models.Post]("ID"),
				builder.Col[models.Post]("Title"),
			).
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Insert post failed: %v", err)
		} else {
			log.Printf("Inserted post: %+v", insertedPosts[0])
		}
	}

	// Example 5: Complex WHERE with AND/OR
	log.Println("\n--- Example 5: Complex WHERE ---")
	complexUsers, err := builder.Select[models.User](qb).
		Where(builder.Gte(builder.Col[models.User]("Age"), 25)).
		And(builder.Like(builder.Col[models.User]("Email"), "%@example.com")).
		All(ctx)
	if err != nil {
		log.Printf("Complex query failed: %v", err)
	} else {
		log.Printf("Found %d users matching complex criteria", len(complexUsers))
	}

	// Example 6: COUNT
	log.Println("\n--- Example 6: COUNT ---")
	userCount, err := builder.Select[models.User](qb).
		Where(builder.Gte(builder.Col[models.User]("Age"), 18)).
		Count(ctx)
	if err != nil {
		log.Printf("Count failed: %v", err)
	} else {
		log.Printf("Total users (18+): %d", userCount)
	}

	// Example 7: DELETE
	log.Println("\n--- Example 7: DELETE ---")
	deleteCount, err := builder.Delete[models.Post](qb).
		Where(builder.Eq(builder.Col[models.Post]("Published"), false)).
		Exec(ctx)
	if err != nil {
		log.Printf("Delete failed: %v", err)
	} else {
		log.Printf("Deleted %d unpublished posts", deleteCount)
	}

	log.Println("\n=== Examples Complete ===")
}
