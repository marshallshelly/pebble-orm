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

	// Example 4: INSERT a post with enum status
	log.Println("\n--- Example 4: INSERT Post with Enum Status ---")
	if len(insertedUsers) > 0 {
		newPost := models.Post{
			Title:    "Getting Started with Pebble ORM",
			Content:  "Pebble ORM is a type-safe PostgreSQL ORM for Go...",
			AuthorID: insertedUsers[0].ID,
			Status:   "published", // Using enum value
		}
		insertedPosts, err := builder.Insert[models.Post](qb).
			Values(newPost).
			Returning(
				builder.Col[models.Post]("ID"),
				builder.Col[models.Post]("Title"),
				builder.Col[models.Post]("Status"),
			).
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Insert post failed: %v", err)
		} else {
			log.Printf("Inserted post: %s (status: %s)", insertedPosts[0].Title, insertedPosts[0].Status)
		}
	}

	// Example 5: SELECT with Relationship (Preload)
	log.Println("\n--- Example 5: SELECT with Relationship (Preload) ---")
	posts, err := builder.Select[models.Post](qb).
		Where(builder.Eq(builder.Col[models.Post]("Status"), "published")).
		Preload("Author").
		All(ctx)
	if err != nil {
		log.Printf("Select with preload failed: %v", err)
	} else {
		log.Printf("Found %d published posts:", len(posts))
		for _, post := range posts {
			authorName := "Unknown"
			if post.Author != nil {
				authorName = post.Author.Name
			}
			log.Printf("  - %s by %s", post.Title, authorName)
		}
	}

	// Example 6: Query by Enum Status
	log.Println("\n--- Example 6: Query by Enum Status ---")
	publishedPosts, err := builder.Select[models.Post](qb).
		Where(builder.Eq(builder.Col[models.Post]("Status"), "published")).
		All(ctx)
	if err != nil {
		log.Printf("Query by enum failed: %v", err)
	} else {
		log.Printf("Found %d published posts:", len(publishedPosts))
		for _, post := range publishedPosts {
			log.Printf("  - %s (status: %s)", post.Title, post.Status)
		}
	}

	// Example 7: COUNT
	log.Println("\n--- Example 7: COUNT ---")
	userCount, err := builder.Select[models.User](qb).
		Where(builder.Gte(builder.Col[models.User]("Age"), 18)).
		Count(ctx)
	if err != nil {
		log.Printf("Count failed: %v", err)
	} else {
		log.Printf("Total users (18+): %d", userCount)
	}

	// Example 8: DELETE by Enum Status
	log.Println("\n--- Example 8: DELETE by Enum Status ---")
	deleteCount, err := builder.Delete[models.Post](qb).
		Where(builder.Eq(builder.Col[models.Post]("Status"), "draft")).
		Exec(ctx)
	if err != nil {
		log.Printf("Delete failed: %v", err)
	} else {
		log.Printf("Deleted %d draft posts", deleteCount)
	}

	log.Println("\n=== Examples Complete ===")
}
