package main

import (
	"context"
	"fmt"
	"log"

	"github.com/marshallshelly/pebble-orm/examples/relationships/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/relationships/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
)

func main() {
	ctx := context.Background()

	log.Println("=== Relationships & Eager Loading Example ===\n")

	// Connect to database
	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("✅ Connected to database\n")

	// Create query builder
	qb := builder.New(db)

	// Example 1: hasMany relationship (Author has many Books)
	fmt.Println("--- Example 1: hasMany (Author → Books) ---")

	author := models.Author{Name: "J.K. Rowling"}
	authorResult, err := builder.Insert[models.Author](qb).
		Values(author).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Fatalf("Failed to create author: %v", err)
	}

	authorID := authorResult[0].ID
	log.Printf("Created author: %s (ID: %d)\n", author.Name, authorID)

	// Create books for the author
	books := []models.Book{
		{Title: "Harry Potter and the Philosopher's Stone", ISBN: "978-0439554930", AuthorID: authorID},
		{Title: "Harry Potter and the Chamber of Secrets", ISBN: "978-0439554923", AuthorID: authorID},
		{Title: "Harry Potter and the Prisoner of Azkaban", ISBN: "978-0439554916", AuthorID: authorID},
	}

	for _, book := range books {
		_, err := builder.Insert[models.Book](qb).Values(book).Exec(ctx)
		if err != nil {
			log.Fatalf("Failed to create book: %v", err)
		}
	}
	log.Printf("Created %d books\n", len(books))

	// Query authors with eager loading of books (prevents N+1 queries)
	fmt.Println("\nQuerying authors with their books (eager loading)...")
	authors, err := builder.Select[models.Author](qb).
		Preload("Books"). // ✅ Eagerly load the Books relationship
		Where(builder.Eq("name", "J.K. Rowling")).
		All(ctx)
	if err != nil {
		log.Fatalf("Failed to query authors: %v", err)
	}

	for _, a := range authors {
		author := a
		fmt.Printf("\n Author: %s\n", author.Name)
		fmt.Printf("  Books (%d):\n", len(author.Books))
		for _, book := range author.Books {
			fmt.Printf("    - %s (ISBN: %s)\n", book.Title, book.ISBN)
		}
	}

	// Example 2: hasOne relationship (User has one Profile)
	fmt.Println("\n--- Example 2: hasOne (User → Profile) ---")

	user := models.User{
		Name:  "Alice Smith",
		Email: "alice@example.com",
	}

	userResult, err := builder.Insert[models.User](qb).
		Values(user).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	userID := userResult[0].ID
	log.Printf("Created user: %s (ID: %d)\n", user.Name, userID)

	// Create profile for the user
	profile := models.Profile{
		Bio:    "Software engineer and book enthusiast",
		Avatar: "https://example.com/avatar.jpg",
		UserID: userID,
	}

	_, err = builder.Insert[models.Profile](qb).Values(profile).Exec(ctx)
	if err != nil {
		log.Fatalf("Failed to create profile: %v", err)
	}
	log.Println("Created profile")

	// Query users with their profiles
	fmt.Println("\nQuerying users with their profiles (eager loading)...")
	users, err := builder.Select[models.User](qb).
		Preload("Profile"). // ✅ Eagerly load the Profile relationship
		Where(builder.Eq("email", "alice@example.com")).
		All(ctx)
	if err != nil {
		log.Fatalf("Failed to query users: %v", err)
	}

	for _, u := range users {
		user := u
		fmt.Printf("\nUser: %s (%s)\n", user.Name, user.Email)
		if user.Profile != nil {
			fmt.Printf("  Profile:\n")
			fmt.Printf("    Bio: %s\n", user.Profile.Bio)
			fmt.Printf("    Avatar: %s\n", user.Profile.Avatar)
		}
	}

	// Example 3: belongsTo relationship (Book belongs to Author)
	fmt.Println("\n--- Example 3: belongsTo (Book → Author) ---")

	fmt.Println("\nQuerying books with their authors (eager loading)...")
	booksResult, err := builder.Select[models.Book](qb).
		Preload("Author"). // ✅ Eagerly load the Author relationship
		Where(builder.Like("title", "%Harry Potter%")).
		OrderByAsc("title").
		All(ctx)
	if err != nil {
		log.Fatalf("Failed to query books: %v", err)
	}

	for _, b := range booksResult {
		book := b
		fmt.Printf("\nBook: %s\n", book.Title)
		if book.Author != nil {
			fmt.Printf("  Author: %s\n", book.Author.Name)
		}
	}

	// Example 4: manyToMany relationship (User has many Roles)
	fmt.Println("\n--- Example 4: manyToMany (User ↔ Roles) ---")

	// Create roles
	roles := []models.Role{
		{Name: "admin"},
		{Name: "editor"},
		{Name: "viewer"},
	}

	for _, role := range roles {
		_, err := builder.Insert[models.Role](qb).Values(role).Exec(ctx)
		if err != nil {
			// Role might already exist
			log.Printf("Note: Role %s might already exist", role.Name)
		}
	}
	log.Printf("Ensured %d roles exist\n", len(roles))

	// Note: In production, you would insert into the user_roles junction table
	// For demonstration purposes, we show the Preload syntax

	fmt.Println("\nQuerying users with their roles (eager loading)...")
	usersWithRoles, err := builder.Select[models.User](qb).
		Preload("Roles"). // ✅ Eagerly load the Roles relationship (many-to-many)
		Where(builder.Eq("email", "alice@example.com")).
		All(ctx)
	if err != nil {
		log.Fatalf("Failed to query users: %v", err)
	}

	for _, u := range usersWithRoles {
		user := u
		fmt.Printf("\nUser: %s\n", user.Name)
		fmt.Printf("  Roles (%d):\n", len(user.Roles))
		for _, role := range user.Roles {
			fmt.Printf("    - %s\n", role.Name)
		}
	}

	// Example 5: Multiple preloads
	fmt.Println("\n--- Example 5: Multiple Preloads ---")

	fmt.Println("\nQuerying with multiple relationships...")
	authorsComplete, err := builder.Select[models.Author](qb).
		Preload("Books"). // Load all books
		Where(builder.Gt("id", 0)).
		All(ctx)
	if err != nil {
		log.Fatalf("Failed to query authors: %v", err)
	}

	fmt.Printf("\n✅ Found %d authors with complete data\n", len(authorsComplete))

	log.Println("\n✅ All relationship examples completed!")
	log.Println("\nKey Takeaways:")
	log.Println("  - Use Preload() to eager load relationships and prevent N+1 queries")
	log.Println("  - hasMany: One parent → Many children (Author → Books)")
	log.Println("  - belongsTo: Child → One parent (Book → Author)")
	log.Println("  - hasOne: One parent → One child (User → Profile)")
	log.Println("  - manyToMany:  Many ↔ Many through junction table (User ↔ Roles)")
}
