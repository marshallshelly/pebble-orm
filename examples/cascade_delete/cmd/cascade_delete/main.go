package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/marshallshelly/pebble-orm/examples/cascade_delete/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/cascade_delete/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
)

func main() {
	ctx := context.Background()

	log.Println("=== CASCADE DELETE & Foreign Key Actions Example ===\n")

	// Connect to database
	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("✅ Connected to database\n")

	// Create query builder
	qb := builder.New(db)

	// Example 1: CASCADE DELETE
	fmt.Println("--- Example 1: CASCADE DELETE (Delete cascades to children) ---")
	if err := example1CascadeDelete(ctx, qb); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Example 2: SET NULL
	fmt.Println("\n--- Example 2: SET NULL (Foreign key set to NULL) ---")
	if err := example2SetNull(ctx, qb); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Example 3: RESTRICT
	fmt.Println("\n--- Example 3: RESTRICT (Prevent deletion if children exist) ---")
	if err := example3Restrict(ctx, qb); err != nil {
		log.Printf("Error: %v\n", err)
	}

	log.Println("\n✅ All cascade delete examples completed!")
	log.Println("\nKey Takeaways:")
	log.Println("  - CASCADE: Automatically deletes child records")
	log.Println("  - SET NULL: Sets foreign key to NULL when parent deleted")
	log.Println("  - RESTRICT: Prevents deletion if children exist")
	log.Println("  - Database enforces these constraints automatically")
}

func example1CascadeDelete(ctx context.Context, qb *builder.DB) error {
	// Create a user
	users, err := builder.Insert[models.User](qb).
		Values(models.User{
			Name:      "Alice",
			Email:     "alice@example.com",
			CreatedAt: time.Now(),
		}).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		return err
	}

	userID := users[0].ID
	fmt.Printf("Created user: %s (ID: %d)\n", users[0].Name, userID)

	// Create posts for the user
	for i := 1; i <= 3; i++ {
		post := models.Post{
			Title:     fmt.Sprintf("Post #%d by Alice", i),
			Content:   fmt.Sprintf("Content for post %d", i),
			AuthorID:  userID,
			CreatedAt: time.Now(),
		}

		posts, err := builder.Insert[models.Post](qb).
			Values(post).
			Returning("*").
			ExecReturning(ctx)
		if err != nil {
			return err
		}

		postID := posts[0].ID
		fmt.Printf("  Created post: %s (ID: %d)\n", post.Title, postID)

		// Add comments to each post
		for j := 1; j <= 2; j++ {
			comment := models.Comment{
				Content:   fmt.Sprintf("Comment %d on post %d", j, i),
				PostID:    postID,
				AuthorID:  &userID,
				CreatedAt: time.Now(),
			}

			_, err := builder.Insert[models.Comment](qb).
				Values(comment).
				Exec(ctx)
			if err != nil {
				return err
			}
		}
	}

	// Count posts and comments before delete
	postCount, _ := builder.Select[models.Post](qb).
		Where(builder.Eq("author_id", userID)).
		Count(ctx)

	commentCount, _ := builder.Select[models.Comment](qb).
		Count(ctx)

	fmt.Printf("\nBefore deleting user:\n")
	fmt.Printf("  Posts by user %d: %d\n", userID, postCount)
	fmt.Printf("  Total comments: %d\n", commentCount)

	// Delete the user - CASCADE will delete posts and comments
	fmt.Printf("\n🗑️  Deleting user %d...\n", userID)
	deleted, err := builder.Delete[models.User](qb).
		Where(builder.Eq("id", userID)).
		Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted %d user\n", deleted)

	// Count posts and comments after delete
	postCountAfter, _ := builder.Select[models.Post](qb).
		Where(builder.Eq("author_id", userID)).
		Count(ctx)

	commentCountAfter, _ := builder.Select[models.Comment](qb).
		Count(ctx)

	fmt.Printf("\nAfter deleting user:\n")
	fmt.Printf("  Posts by user %d: %d ✅ (CASCADE deleted)\n", userID, postCountAfter)
	fmt.Printf("  Total comments: %d ✅ (CASCADE deleted via posts)\n", commentCountAfter)

	return nil
}

func example2SetNull(ctx context.Context, qb *builder.DB) error {
	// Create two users
	user1, err := builder.Insert[models.User](qb).
		Values(models.User{
			Name:      "Bob",
			Email:     "bob@example.com",
			CreatedAt: time.Now(),
		}).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		return err
	}
	bobID := user1[0].ID

	user2, err := builder.Insert[models.User](qb).
		Values(models.User{
			Name:      "Charlie",
			Email:     "charlie@example.com",
			CreatedAt: time.Now(),
		}).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		return err
	}
	charlieID := user2[0].ID

	fmt.Printf("Created users: Bob (ID: %d), Charlie (ID: %d)\n", bobID, charlieID)

	// Bob creates a post
	bobPost, err := builder.Insert[models.Post](qb).
		Values(models.Post{
			Title:     "Bob's Post",
			Content:   "Content by Bob",
			AuthorID:  bobID,
			CreatedAt: time.Now(),
		}).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		return err
	}
	bobPostID := bobPost[0].ID

	fmt.Printf("Bob created post (ID: %d)\n", bobPostID)

	// Charlie comments on Bob's post
	_, err = builder.Insert[models.Comment](qb).
		Values(models.Comment{
			Content:   "Great post, Bob!",
			PostID:    bobPostID,
			AuthorID:  &charlieID,
			CreatedAt: time.Now(),
		}).
		Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Charlie commented on Bob's post\n")

	// Count comments by Charlie
	commentCount, _ := builder.Select[models.Comment](qb).
		Where(builder.Eq("author_id", charlieID)).
		Count(ctx)

	fmt.Printf("\nBefore deleting Charlie:\n")
	fmt.Printf("  Comments by Charlie: %d\n", commentCount)

	// Delete Charlie - comments will have author_id SET NULL
	fmt.Printf("\n🗑️  Deleting Charlie...\n")
	_, err = builder.Delete[models.User](qb).
		Where(builder.Eq("id", charlieID)).
		Exec(ctx)
	if err != nil {
		return err
	}

	// Count comments with NULL author_id
	nullAuthorComments, _ := builder.Select[models.Comment](qb).
		Where(builder.IsNull("author_id")).
		Count(ctx)

	fmt.Printf("\nAfter deleting Charlie:\n")
	fmt.Printf("  Comments with NULL author: %d ✅ (SET NULL applied)\n", nullAuthorComments)
	fmt.Printf("  Comments still exist, just author is NULL\n")

	return nil
}

func example3Restrict(ctx context.Context, qb *builder.DB) error {
	// Create a category
	categories, err := builder.Insert[models.Category](qb).
		Values(models.Category{
			Name: "Electronics",
		}).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		return err
	}
	categoryID := categories[0].ID

	fmt.Printf("Created category: Electronics (ID: %d)\n", categoryID)

	// Create products in this category
	for i := 1; i <= 2; i++ {
		product := models.Product{
			Name:       fmt.Sprintf("Product %d", i),
			Price:      99.99 * float64(i),
			CategoryID: categoryID,
		}

		_, err := builder.Insert[models.Product](qb).
			Values(product).
			Exec(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("  Created product: %s\n", product.Name)
	}

	// Try to delete the category - should fail because of RESTRICT
	fmt.Printf("\n🗑️  Attempting to delete category %d (has products)...\n", categoryID)

	_, err = builder.Delete[models.Category](qb).
		Where(builder.Eq("id", categoryID)).
		Exec(ctx)

	if err != nil {
		fmt.Printf("❌ Deletion prevented by RESTRICT constraint!\n")
		fmt.Printf("Error: %v\n", err)
		fmt.Printf("✅ This is correct behavior - can't delete category with products\n")
	} else {
		fmt.Printf("⚠️  Unexpected: Category was deleted (RESTRICT should have prevented this)\n")
	}

	// To delete the category, we need to delete products first
	fmt.Printf("\n🗑️  Deleting all products in category first...\n")
	deleted, _ := builder.Delete[models.Product](qb).
		Where(builder.Eq("category_id", categoryID)).
		Exec(ctx)

	fmt.Printf("Deleted %d products\n", deleted)

	// Now we can delete the category
	fmt.Printf("🗑️  Now deleting category...\n")
	_, err = builder.Delete[models.Category](qb).
		Where(builder.Eq("id", categoryID)).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	fmt.Printf("✅ Category deleted successfully (no products left)\n")

	return nil
}
