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

	// Connect to database
	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("✅ Connected to database successfully")

	// Create query builder
	qb := builder.New(db)

	// Example 1: INSERT a new user
	log.Println("\n--- Example 1: INSERT ---")
	newUser := models.User{
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   28,
	}

	insertedUsers, err := builder.Insert[models.User](qb).
		Values(newUser).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Printf("Insert failed: %v", err)
	} else {
		log.Printf("Inserted user: %+v", insertedUsers[0])
	}

	// Example 2: SELECT with WHERE
	log.Println("\n--- Example 2: SELECT with WHERE ---")
	users, err := builder.Select[models.User](qb).
		Where(builder.Gte("age", 18)).
		OrderByDesc("created_at").
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
		Set("age", 29).
		Where(builder.Eq("email", "alice@example.com")).
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
			Returning("*").
			ExecReturning(ctx)
		if err != nil {
			log.Printf("Insert post failed: %v", err)
		} else {
			log.Printf("Inserted post: %s", insertedPosts[0].Title)
		}
	}

	// Example 5: SELECT posts with author (relationship)
	log.Println("\n--- Example 5: SELECT with Relationship ---")
	posts, err := builder.Select[models.Post](qb).
		Preload("Author"). // Eager load author
		Where(builder.Eq("published", true)).
		All(ctx)
	if err != nil {
		log.Printf("Select posts failed: %v", err)
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

	// Example 6: COUNT
	log.Println("\n--- Example 6: COUNT ---")
	totalUsers, err := builder.Select[models.User](qb).Count(ctx)
	if err != nil {
		log.Printf("Count failed: %v", err)
	} else {
		log.Printf("Total users in database: %d", totalUsers)
	}

	// Example 7: DELETE
	log.Println("\n--- Example 7: DELETE ---")
	deletedCount, err := builder.Delete[models.Post](qb).
		Where(builder.Eq("published", false)).
		Exec(ctx)
	if err != nil {
		log.Printf("Delete failed: %v", err)
	} else {
		log.Printf("Deleted %d unpublished posts", deletedCount)
	}

	log.Println("\n✅ All examples completed!")
}
