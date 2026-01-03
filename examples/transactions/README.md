# Transactions Example

This example demonstrates **transaction handling** in Pebble ORM including:

- ✅ **Type-Safe Transactions** - Full query builder support within transactions
- ✅ **Basic Transactions** - Commit and rollback
- ✅ **Error Handling** - Automatic rollback on errors
- ✅ **Atomicity** - All-or-nothing operations
- ✅ **Money Transfers** - Real-world atomic operations with row locking
- ✅ **Savepoints** - Nested transaction control

## Features Demonstrated

### 1. Basic Transaction with Commit

```go
// Begin transaction
tx, err := qb.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback()  // Safety net - rollback if we don't reach Commit

// Execute operations using type-safe builders
account1 := models.Account{UserID: 1, Balance: 1000.00}
account2 := models.Account{UserID: 2, Balance: 500.00}

inserted, err := builder.TxInsert[models.Account](tx).
    Values(account1, account2).
    Returning("*").
    ExecReturning()
if err != nil {
    return err  // Defer will rollback
}

// Commit if successful
if err := tx.Commit(); err != nil {
    return err
}
```

### 2. Automatic Rollback on Error

```go
tx, err := qb.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback()  // Executes if Commit not called

account := models.Account{UserID: 999, Balance: 10000.00}

_, err = builder.TxInsert[models.Account](tx).
    Values(account).
    ExecReturning()
if err != nil {
    return err  // Defer will automatically rollback
}

tx.Commit()  // Success - defer won't rollback
```

### 3. Atomic Money Transfer with Row Locking

```go
tx, err := qb.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback()

// Fetch current balances with FOR UPDATE lock (prevents race conditions)
fromAccount, err := builder.TxSelect[models.Account](tx).
    Where(builder.Eq("id", fromAccountID)).
    ForUpdate().  // Lock row
    First()
if err != nil {
    return err
}

toAccount, err := builder.TxSelect[models.Account](tx).
    Where(builder.Eq("id", toAccountID)).
    ForUpdate().  // Lock row
    First()
if err != nil {
    return err
}

// Check sufficient balance
if fromAccount.Balance < transferAmount {
    return fmt.Errorf("insufficient balance")
}

// Update balances
_, err = builder.TxUpdate[models.Account](tx).
    Set("balance", fromAccount.Balance - transferAmount).
    Where(builder.Eq("id", fromAccountID)).
    Exec()
if err != nil {
    return err
}

_, err = builder.TxUpdate[models.Account](tx).
    Set("balance", toAccount.Balance + transferAmount).
    Where(builder.Eq("id", toAccountID)).
    Exec()
if err != nil {
    return err
}

// Both updates succeed or both fail
tx.Commit()
```

### 4. Savepoints (Nested Transactions)

```go
tx, err := qb.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback()

// First operation
account1 := models.Account{UserID: 300, Balance: 1000.00}
inserted1, err := builder.TxInsert[models.Account](tx).
    Values(account1).
    ExecReturning()
if err != nil {
    return err
}

// Create a savepoint
if err := tx.Savepoint("before_second_account"); err != nil {
    return err
}

// Second operation (might fail)
account2 := models.Account{UserID: 400, Balance: 500.00}
_, err = builder.TxInsert[models.Account](tx).
    Values(account2).
    ExecReturning()

// Rollback to savepoint if second operation fails
if err != nil {
    tx.RollbackToSavepoint("before_second_account")
    // First account still exists
}

// Commit transaction (only first account if second failed)
tx.Commit()
```

## Running the Example

### Prerequisites

- PostgreSQL running on `localhost:5432`
- Database: `pebble_transactions`

```bash
# Create database
createdb pebble_transactions

# Run the example
cd examples/transactions
go run cmd/transactions/main.go
```

## Project Structure

```
transactions/
├── cmd/
│   └── transactions/
│       └── main.go           # Main application with 4 examples
├── internal/
│   ├── database/
│   │   └── db.go             # Database connection
│   └── models/
│       ├── models.go         # Account model
│       └── registry.go       # Model registration
├── go.mod
└── README.md
```

## Models

### Account

```go
type Account struct {
    ID      int     `po:"id,primaryKey,serial"`
    UserID  int     `po:"user_id,integer,notNull"`
    Balance float64 `po:"balance,double precision,notNull,default(0)"`
}
```

## Example Output

```
=== Transactions & Atomicity Example ===

✅ Connected to database

--- Example 1: Basic Transaction (Commit) ---
✅ Transaction committed successfully
  - Created account 1 with balance: $1000.00
  - Created account 2 with balance: $500.00

--- Example 2: Transaction Rollback ---
Created account 999 with balance: $10000.00
Simulating error - rolling back transaction...
Accounts with user_id=999 after rollback: 0 (should be 0)

--- Example 3: Money Transfer (Atomic) ---
Initial state:
  From account (ID 3): $1000.00
  To account (ID 4): $500.00
  Transfer amount: $250.00

✅ Successfully transferred $250.00
  From account ID: 3
  To account ID: 4

Final balances:
  From account: $750.00 (deducted $250.00)
  To account: $750.00 (added $250.00)

--- Example 4: Savepoints (Nested Transactions) ---
Created account 5 with balance: $1000.00
Created savepoint: before_second_account
Created account 6 with balance: $500.00
Simulating error - rolling back to savepoint...
Rolled back to savepoint - second account creation undone

Result after savepoint rollback:
  Accounts with user_id=300: 1 (should be 1)
  Accounts with user_id=400: 0 (should be 0)

✅ All transaction examples completed!

Key Takeaways:
  - Transactions ensure atomicity (all-or-nothing)
  - Use defer tx.Rollback() as safety net
  - Type-safe query builders work within transactions
  - Savepoints allow nested transaction control
  - Perfect for money transfers, inventory updates, etc.
```

## Transaction API

### Begin Transaction

```go
tx, err := qb.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback()  // Always defer rollback as safety net
```

### Type-Safe Query Builders

All query builders work within transactions using generic functions:

```go
// INSERT
inserted, err := builder.TxInsert[User](tx).
    Values(user).
    ExecReturning()

// SELECT
users, err := builder.TxSelect[User](tx).
    Where(builder.Eq("status", "active")).
    All()

// UPDATE
count, err := builder.TxUpdate[User](tx).
    Set("status", "active").
    Where(builder.Eq("id", userId)).
    Exec()

// DELETE
count, err := builder.TxDelete[User](tx).
    Where(builder.Eq("status", "inactive")).
    Exec()
```

### Row Locking with FOR UPDATE

```go
// Lock row for update to prevent race conditions
account, err := builder.TxSelect[Account](tx).
    Where(builder.Eq("id", accountID)).
    ForUpdate().  // Locks the row until transaction commits
    First()
```

### Savepoints

```go
// Create savepoint
tx.Savepoint("my_savepoint")

// Rollback to savepoint
tx.RollbackToSavepoint("my_savepoint")

// Release savepoint (commit it)
tx.ReleaseSavepoint("my_savepoint")
```

### Commit and Rollback

```go
// Commit transaction (all changes persisted)
if err := tx.Commit(); err != nil {
    return err
}

// Explicit rollback (usually handled by defer)
if err := tx.Rollback(); err != nil {
    return err
}
```

## Transaction Best Practices

### ✅ DO:

- Always use `defer tx.Rollback()` immediately after Begin
- Keep transactions short-lived
- Handle all errors properly
- Use `ForUpdate()` for row locking when needed
- Use savepoints for complex nested operations
- Test rollback behavior

### ❌ DON'T:

- Forget to defer rollback
- Leave transactions open for too long
- Mix transactional and non-transactional code
- Ignore transaction errors
- Forget to lock rows in concurrent scenarios

## Error Handling Patterns

### Pattern 1: Early Return with Defer

```go
tx, err := qb.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback()  // Runs on any early return

result1, err := builder.TxInsert[Account](tx).Values(account1).ExecReturning()
if err != nil {
    return err  // Rollback happens automatically
}

result2, err := builder.TxInsert[Account](tx).Values(account2).ExecReturning()
if err != nil {
    return err  // Rollback happens automatically
}

// Only commit if everything succeeded
return tx.Commit()
```

### Pattern 2: Conditional Rollback

```go
tx, err := qb.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback()

// Try operation
_, err = builder.TxUpdate[Account](tx).
    Set("balance", newBalance).
    Where(builder.Eq("id", accountID)).
    Exec()

if err != nil {
    log.Printf("Operation failed, rolling back: %v", err)
    return err  // Defer handles rollback
}

return tx.Commit()
```

### Pattern 3: Explicit Error Handling

```go
tx, err := qb.Begin(ctx)
if err != nil {
    return fmt.Errorf("failed to begin transaction: %w", err)
}
defer tx.Rollback()

inserted, err := builder.TxInsert[Account](tx).
    Values(account).
    ExecReturning()
if err != nil {
    return fmt.Errorf("failed to insert account: %w", err)
}

if err := tx.Commit(); err != nil {
    return fmt.Errorf("failed to commit transaction: %w", err)
}

return nil
```

## Common Use Cases

### 1. Money Transfer

```go
// Atomic transfer between accounts with race condition prevention
tx, _ := qb.Begin(ctx)
defer tx.Rollback()

// Lock both accounts
from, _ := builder.TxSelect[Account](tx).Where(builder.Eq("id", fromID)).ForUpdate().First()
to, _ := builder.TxSelect[Account](tx).Where(builder.Eq("id", toID)).ForUpdate().First()

// Validate and update
if from.Balance < amount {
    return errors.New("insufficient funds")
}

builder.TxUpdate[Account](tx).Set("balance", from.Balance - amount).Where(builder.Eq("id", fromID)).Exec()
builder.TxUpdate[Account](tx).Set("balance", to.Balance + amount).Where(builder.Eq("id", toID)).Exec()

tx.Commit()
```

### 2. Inventory Update

```go
// Atomic inventory deduction with stock check
tx, _ := qb.Begin(ctx)
defer tx.Rollback()

product, _ := builder.TxSelect[Product](tx).Where(builder.Eq("id", productID)).ForUpdate().First()

if product.Stock < quantity {
    return errors.New("out of stock")
}

builder.TxUpdate[Product](tx).Set("stock", product.Stock - quantity).Where(builder.Eq("id", productID)).Exec()
builder.TxInsert[Order](tx).Values(order).ExecReturning()

tx.Commit()
```

### 3. Cascading Updates

```go
// Update user and all related records atomically
tx, _ := qb.Begin(ctx)
defer tx.Rollback()

builder.TxUpdate[User](tx).Set("status", "inactive").Where(builder.Eq("id", userID)).Exec()
builder.TxUpdate[Post](tx).Set("published", false).Where(builder.Eq("user_id", userID)).Exec()
builder.TxDelete[Session](tx).Where(builder.Eq("user_id", userID)).Exec()

tx.Commit()
```

## Learn More

- **PostgreSQL Transactions**: https://www.postgresql.org/docs/current/tutorial-transactions.html
- **Isolation Levels**: https://www.postgresql.org/docs/current/transaction-iso.html
- **pgx Documentation**: https://pkg.go.dev/github.com/jackc/pgx/v5

## Next Steps

- Try the [Basic Example](../basic/) for CRUD operations
- See [Relationships Example](../relationships) for joins and eager loading
- Check [Migrations Example](../migrations) for schema management
