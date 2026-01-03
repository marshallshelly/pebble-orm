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

	// Example 1: INSERT with RETURNING (including JSONB fields)
	log.Println("\n--- Example 1: INSERT with RETURNING (JSONB Support) ---")
	newUsers := []models.User{
		{
			Name:  "Alice Smith",
			Email: "alice@example.com",
			Age:   28,
			Preferences: &models.UserPreferences{
				Theme:              "dark",
				EmailNotifications: true,
				Language:           "en",
				FavoriteTopics:     []string{"golang", "databases", "cloud"},
			},
		},
		{
			Name:  "Bob Johnson",
			Email: "bob@example.com",
			Age:   35,
			Preferences: &models.UserPreferences{
				Theme:              "light",
				EmailNotifications: false,
				Language:           "es",
				FavoriteTopics:     []string{"javascript", "react"},
			},
		},
	}
	insertedUsers, err := builder.Insert[models.User](qb).
		Values(newUsers...).
		Returning(
			builder.Col[models.User]("ID"),
			builder.Col[models.User]("Name"),
			builder.Col[models.User]("Email"),
			builder.Col[models.User]("Preferences"),
		).
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert failed: %v", err)
	} else {
		log.Printf("Inserted user: %s with preferences: theme=%s, lang=%s",
			insertedUsers[0].Name,
			insertedUsers[0].Preferences.Theme,
			insertedUsers[0].Preferences.Language)
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

	// Example 4: INSERT a post with enum status and JSONB metadata
	log.Println("\n--- Example 4: INSERT Post with Enum Status and JSONB Metadata ---")
	if len(insertedUsers) > 0 {
		newPost := models.Post{
			Title:    "Getting Started with Pebble ORM",
			Content:  "Pebble ORM is a type-safe PostgreSQL ORM for Go...",
			AuthorID: insertedUsers[0].ID,
			Status:   "published",
			Metadata: &models.PostMetadata{
				Tags:            []string{"golang", "orm", "postgresql", "tutorial"},
				ReadTimeMinutes: 5,
				FeaturedImage:   "https://example.com/images/pebble-intro.jpg",
				SEOKeywords:     []string{"go orm", "postgresql go", "type-safe orm"},
			},
		}
		insertedPosts, err := builder.Insert[models.Post](qb).
			Values(newPost).
			Returning(
				builder.Col[models.Post]("ID"),
				builder.Col[models.Post]("Title"),
				builder.Col[models.Post]("Status"),
				builder.Col[models.Post]("Metadata"),
			).
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Insert post failed: %v", err)
		} else {
			log.Printf("Inserted post: %s (status: %s, tags: %v)",
				insertedPosts[0].Title,
				insertedPosts[0].Status,
				insertedPosts[0].Metadata.Tags)
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

	// Example 8: Query JSONB with operators
	log.Println("\n--- Example 8: Query JSONB with Operators ---")
	// Find users who have "golang" in their favorite topics
	goLangUsers, err := builder.Select[models.User](qb).
		Where(builder.JSONBContains(
			builder.Col[models.User]("Preferences"),
			`{"favoriteTopics": ["golang"]}`,
		)).
		All(ctx)
	if err != nil {
		log.Printf("JSONB query failed: %v", err)
	} else {
		log.Printf("Found %d users interested in golang:", len(goLangUsers))
		for _, user := range goLangUsers {
			if user.Preferences != nil {
				log.Printf("  - %s (topics: %v)", user.Name, user.Preferences.FavoriteTopics)
			}
		}
	}

	// Example 9: DELETE by Enum Status
	log.Println("\n--- Example 9: DELETE by Enum Status ---")
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
