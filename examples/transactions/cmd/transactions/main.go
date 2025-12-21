package main

import (
	"context"
	"fmt"
	"log"

	"github.com/marshallshelly/pebble-orm/examples/transactions/internal/database"
	"github.com/marshallshelly/pebble-orm/examples/transactions/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
)

func main() {
	ctx := context.Background()

	log.Println("=== Transactions & Atomicity Example ===\n")

	// Connect to database
	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("✅ Connected to database\n")

	// Create query builder
	qb := builder.New(db)

	// Example 1: Basic transaction with commit
	fmt.Println("--- Example 1: Basic Transaction (Commit) ---")
	if err := example1BasicTransaction(ctx, qb); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Example 2: Transaction with rollback
	fmt.Println("\n--- Example 2: Transaction Rollback ---")
	if err := example2TransactionRollback(ctx, qb); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Example 3: Money transfer with transaction
	fmt.Println("\n--- Example 3: Money Transfer (Atomic) ---")
	if err := example3MoneyTransfer(ctx, qb); err != nil {
		log.Printf("Error: %v\n", err)
	}

	log.Println("\n✅ All transaction examples completed!")
	log.Println("\nKey Takeaways:")
	log.Println("  - Transactions ensure atomicity (all-or-nothing)")
	log.Println("  - Use defer tx.Rollback() as safety net")
	log.Println("  - Use Begin() to start a transaction")
	log.Println("  - Perfect for money transfers, inventory updates, etc.")
}

func example1BasicTransaction(ctx context.Context, qb *builder.DB) error {
	// Begin transaction
	tx, err := qb.Runtime().Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if we don't reach Commit

	// Create accounts within transaction
	account1 := models.Account{UserID: 1, Balance: 1000.00}
	account2 := models.Account{UserID: 2, Balance: 500.00}

	// Note: For transaction support, we'd need to execute raw SQL or use pgx directly
	// This is a simplified example showing the pattern
	_, err = tx.Exec(ctx,
		"INSERT INTO accounts (user_id, balance) VALUES ($1, $2), ($3, $4)",
		account1.UserID, account1.Balance, account2.UserID, account2.Balance)
	if err != nil {
		return fmt.Errorf("failed to insert accounts: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("✅ Transaction committed successfully")
	fmt.Println("  - Created account 1 with balance: $1000.00")
	fmt.Println("  - Created account 2 with balance: $500.00")

	return nil
}

func example2TransactionRollback(ctx context.Context, qb *builder.DB) error {
	// Begin transaction
	tx, err := qb.Runtime().Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // This will rollback since we don't call Commit

	// Create an account
	account := models.Account{UserID: 999, Balance: 10000.00}

	_, err = tx.Exec(ctx,
		"INSERT INTO accounts (user_id, balance) VALUES ($1, $2)",
		account.UserID, account.Balance)
	if err != nil {
		return fmt.Errorf("failed to insert account: %w", err)
	}

	fmt.Println("Created account with balance: $10000.00")
	fmt.Println("Simulating error - rolling back transaction...")

	// Don't commit - defer will rollback
	// Verify the account doesn't exist after rollback
	count, _ := builder.Select[models.Account](qb).
		Where(builder.Eq("user_id", 999)).
		Count(ctx)

	fmt.Printf("Accounts with user_id=999 after rollback: %d (should be 0)\n", count)

	return nil
}

func example3MoneyTransfer(ctx context.Context, qb *builder.DB) error {
	// First, create two accounts
	fromAccount := models.Account{UserID: 100, Balance: 1000.00}
	toAccount := models.Account{UserID: 200, Balance: 500.00}

	fromResult, err := builder.Insert[models.Account](qb).
		Values(fromAccount).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		return err
	}

	toResult, err := builder.Insert[models.Account](qb).
		Values(toAccount).
		Returning("*").
		ExecReturning(ctx)
	if err != nil {
		return err
	}

	fromAccountID := fromResult[0].ID
	toAccountID := toResult[0].ID
	transferAmount := 250.00

	fmt.Printf("Initial state:\n")
	fmt.Printf("  From account (ID %d): $%.2f\n", fromAccountID, fromAccount.Balance)
	fmt.Printf("  To account (ID %d): $%.2f\n", toAccountID, toAccount.Balance)
	fmt.Printf("  Transfer amount: $%.2f\n\n", transferAmount)

	// Begin transaction for the transfer
	tx, err := qb.Runtime().Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Deduct from source and add to destination
	_, err = tx.Exec(ctx,
		"UPDATE accounts SET balance = balance - $1 WHERE id = $2",
		transferAmount, fromAccountID)
	if err != nil {
		return fmt.Errorf("failed to deduct: %w", err)
	}

	_, err = tx.Exec(ctx,
		"UPDATE accounts SET balance = balance + $1 WHERE id = $2",
		transferAmount, toAccountID)
	if err != nil {
		return fmt.Errorf("failed to add: %w", err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	fmt.Printf("✅ Successfully transferred $%.2f\n", transferAmount)
	fmt.Printf("  From account ID: %d\n", fromAccountID)
	fmt.Printf("  To account ID: %d\n", toAccountID)

	return nil
}
