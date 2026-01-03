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

	// Example 4: Savepoints (nested transactions)
	fmt.Println("\n--- Example 4: Savepoints (Nested Transactions) ---")
	if err := example4Savepoints(ctx, qb); err != nil {
		log.Printf("Error: %v\n", err)
	}

	log.Println("\n✅ All transaction examples completed!")
	log.Println("\nKey Takeaways:")
	log.Println("  - Transactions ensure atomicity (all-or-nothing)")
	log.Println("  - Use defer tx.Rollback() as safety net")
	log.Println("  - Type-safe query builders work within transactions")
	log.Println("  - Savepoints allow nested transaction control")
	log.Println("  - Perfect for money transfers, inventory updates, etc.")
}

func example1BasicTransaction(ctx context.Context, qb *builder.DB) error {
	// Begin transaction using builder API
	tx, err := qb.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't reach Commit

	// Create accounts within transaction using type-safe builder
	account1 := models.Account{UserID: 1, Balance: 1000.00}
	account2 := models.Account{UserID: 2, Balance: 500.00}

	// Insert using transaction query builder
	inserted, err := builder.TxInsert[models.Account](tx).
		Values(account1, account2).
		Returning("id", "user_id", "balance").
		ExecReturning()
	if err != nil {
		return fmt.Errorf("failed to insert accounts: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("✅ Transaction committed successfully")
	for _, acc := range inserted {
		fmt.Printf("  - Created account %d with balance: $%.2f\n", acc.ID, acc.Balance)
	}

	return nil
}

func example2TransactionRollback(ctx context.Context, qb *builder.DB) error {
	// Begin transaction
	tx, err := qb.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // This will rollback since we don't call Commit

	// Create an account
	account := models.Account{UserID: 999, Balance: 10000.00}

	inserted, err := builder.TxInsert[models.Account](tx).
		Values(account).
		Returning("id", "balance").
		ExecReturning()
	if err != nil {
		return fmt.Errorf("failed to insert account: %w", err)
	}

	fmt.Printf("Created account %d with balance: $%.2f\n", inserted[0].ID, inserted[0].Balance)
	fmt.Println("Simulating error - rolling back transaction...")

	// Don't commit - defer will rollback
	// Verify the account doesn't exist after rollback
	count, _ := builder.Select[models.Account](qb).
		Where(builder.Eq(builder.Col[models.Account]("UserID"), 999)).
		Count(ctx)

	fmt.Printf("Accounts with user_id=999 after rollback: %d (should be 0)\n", count)

	return nil
}

func example3MoneyTransfer(ctx context.Context, qb *builder.DB) error {
	// First, create two accounts outside the transfer transaction
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
	tx, err := qb.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Fetch current balances within transaction (with FOR UPDATE lock)
	currentFrom, err := builder.TxSelect[models.Account](tx).
		Where(builder.Eq("id", fromAccountID)).
		ForUpdate().
		First()
	if err != nil {
		return fmt.Errorf("failed to fetch source account: %w", err)
	}

	currentTo, err := builder.TxSelect[models.Account](tx).
		Where(builder.Eq("id", toAccountID)).
		ForUpdate().
		First()
	if err != nil {
		return fmt.Errorf("failed to fetch destination account: %w", err)
	}

	// Check sufficient balance
	if currentFrom.Balance < transferAmount {
		return fmt.Errorf("insufficient balance: have $%.2f, need $%.2f", currentFrom.Balance, transferAmount)
	}

	// Calculate new balances
	newFromBalance := currentFrom.Balance - transferAmount
	newToBalance := currentTo.Balance + transferAmount

	// Update source account
	_, err = builder.TxUpdate[models.Account](tx).
		Set("balance", newFromBalance).
		Where(builder.Eq("id", fromAccountID)).
		Exec()
	if err != nil {
		return fmt.Errorf("failed to deduct from source: %w", err)
	}

	// Update destination account
	_, err = builder.TxUpdate[models.Account](tx).
		Set("balance", newToBalance).
		Where(builder.Eq("id", toAccountID)).
		Exec()
	if err != nil {
		return fmt.Errorf("failed to add to destination: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	fmt.Printf("✅ Successfully transferred $%.2f\n", transferAmount)
	fmt.Printf("  From account ID: %d\n", fromAccountID)
	fmt.Printf("  To account ID: %d\n", toAccountID)

	// Verify final balances
	finalFrom, _ := builder.Select[models.Account](qb).
		Where(builder.Eq(builder.Col[models.Account]("ID"), fromAccountID)).
		First(ctx)
	finalTo, _ := builder.Select[models.Account](qb).
		Where(builder.Eq(builder.Col[models.Account]("ID"), toAccountID)).
		First(ctx)

	fmt.Printf("\nFinal balances:\n")
	fmt.Printf("  From account: $%.2f (deducted $%.2f)\n", finalFrom.Balance, transferAmount)
	fmt.Printf("  To account: $%.2f (added $%.2f)\n", finalTo.Balance, transferAmount)

	return nil
}

func example4Savepoints(ctx context.Context, qb *builder.DB) error {
	// Begin transaction
	tx, err := qb.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create first account
	account1 := models.Account{UserID: 300, Balance: 1000.00}
	inserted1, err := builder.TxInsert[models.Account](tx).
		Values(account1).
		Returning("*").
		ExecReturning()
	if err != nil {
		return fmt.Errorf("failed to insert first account: %w", err)
	}

	fmt.Printf("Created account %d with balance: $%.2f\n", inserted1[0].ID, inserted1[0].Balance)

	// Create a savepoint
	if err := tx.Savepoint("before_second_account"); err != nil {
		return fmt.Errorf("failed to create savepoint: %w", err)
	}

	fmt.Println("Created savepoint: before_second_account")

	// Create second account
	account2 := models.Account{UserID: 400, Balance: 500.00}
	inserted2, err := builder.TxInsert[models.Account](tx).
		Values(account2).
		Returning("*").
		ExecReturning()
	if err != nil {
		return fmt.Errorf("failed to insert second account: %w", err)
	}

	fmt.Printf("Created account %d with balance: $%.2f\n", inserted2[0].ID, inserted2[0].Balance)

	// Simulate error - rollback to savepoint
	fmt.Println("Simulating error - rolling back to savepoint...")
	if err := tx.RollbackToSavepoint("before_second_account"); err != nil {
		return fmt.Errorf("failed to rollback to savepoint: %w", err)
	}

	fmt.Println("Rolled back to savepoint - second account creation undone")

	// Commit the transaction (only first account should exist)
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// Verify: first account exists, second doesn't
	count300, _ := builder.Select[models.Account](qb).
		Where(builder.Eq(builder.Col[models.Account]("UserID"), 300)).
		Count(ctx)
	count400, _ := builder.Select[models.Account](qb).
		Where(builder.Eq(builder.Col[models.Account]("UserID"), 400)).
		Count(ctx)

	fmt.Printf("\nResult after savepoint rollback:\n")
	fmt.Printf("  Accounts with user_id=300: %d (should be 1)\n", count300)
	fmt.Printf("  Accounts with user_id=400: %d (should be 0)\n", count400)

	return nil
}
