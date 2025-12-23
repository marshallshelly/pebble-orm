# Transactions Example

This example demonstrates **transaction handling** in Pebble ORM including:

- ✅ **Basic Transactions** - Commit and rollback
- ✅ **Transaction Lifecycle** - Begin, commit, rollback
- ✅ **Error Handling** - Automatic rollback on errors
- ✅ **Atomicity** - All-or-nothing operations
- ✅ **Money Transfers** - Real-world atomic operations

## Features Demonstrated

### 1. Basic Transaction with Commit

```go
// Begin transaction
tx, _ := qb.Runtime().Begin(ctx)
defer tx.Rollback(ctx)  // Safety net

// Execute operations
_, _ = tx.Exec(ctx, "INSERT INTO accounts (...) VALUES (...)")

// Commit if successful
tx.Commit(ctx)
```

### 2. Automatic Rollback on Error

```go
tx, _ := qb.Runtime().Begin(ctx)
defer tx.Rollback(ctx)  // Executes if Commit not called

_, err := tx.Exec(ctx, "INSERT ...")
if err != nil {
    return err  // Defer will rollback
}

tx.Commit(ctx)  // Success - defer won't rollback
```

### 3. Atomic Money Transfer

```go
tx, _ := qb.Runtime().Begin(ctx)
defer tx.Rollback(ctx)

// Deduct from source
tx.Exec(ctx, "UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)

// Add to destination
tx.Exec(ctx, "UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)

// Both succeed or both fail
tx.Commit(ctx)
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
│       └── main.go           # Main application
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
Created account with balance: $10000.00
Simulating error - rolling back transaction...

Accounts with user_id=999 after rollback: 0 (should be 0)

--- Example 3: Money Transfer (Atomic) ---
Initial state:
  From account (ID 1): $1000.00
  To account (ID 2): $500.00
  Transfer amount: $250.00

✅ Successfully transferred $250.00
  From account ID: 1
  To account ID: 2

✅ All transaction examples completed!

Key Takeaways:
  - Transactions ensure atomicity (all-or-nothing)
  - Use defer tx.Rollback() as safety net
  - Use Begin() to start a transaction
  - Perfect for money transfers, inventory updates, etc.
```

## Transaction Lifecycle

### 1. Begin Transaction

```go
tx, err := qb.Runtime().Begin(ctx)
if err != nil {
    return err
}
```

### 2. Defer Rollback (Safety Net)

```go
defer tx.Rollback(ctx)
// Will rollback if Commit not called
// Safe to call even after Commit
```

### 3. Execute Operations

```go
_, err = tx.Exec(ctx, "INSERT ...")
if err != nil {
    return err  // Defer will rollback
}

_, err = tx.Exec(ctx, "UPDATE ...")
if err != nil {
    return err  // Defer will rollback
}
```

### 4. Commit on Success

```go
if err := tx.Commit(ctx); err != nil {
    return err
}
// Defer rollback won't execute (Commit called)
```

## Common Use Cases

### Money Transfer

```go
func transferMoney(ctx context.Context, db *runtime.DB, fromID, toID int64, amount float64) error {
    tx, _ := db.Begin(ctx)
    defer tx.Rollback(ctx)

    // Deduct from source
    _, err := tx.Exec(ctx,
        "UPDATE accounts SET balance = balance - $1 WHERE id = $2",
        amount, fromID)
    if err != nil {
        return err
    }

    // Add to destination
    _, err = tx.Exec(ctx,
        "UPDATE accounts SET balance = balance + $1 WHERE id = $2",
        amount, toID)
    if err != nil {
        return err
    }

    return tx.Commit(ctx)
}
```

### Order Creation

```go
func createOrder(ctx context.Context, db *runtime.DB, order Order) error {
    tx, _ := db.Begin(ctx)
    defer tx.Rollback(ctx)

    // Create order
    _, err := tx.Exec(ctx, "INSERT INTO orders (...) VALUES (...)")
    if err != nil {
        return err
    }

    // Create line items
    for _, item := range order.Items {
        _, err := tx.Exec(ctx, "INSERT INTO line_items (...) VALUES (...)")
        if err != nil {
            return err
        }
    }

    // Update inventory
    _, err = tx.Exec(ctx, "UPDATE products SET stock = stock - 1 ...")
    if err != nil {
        return err
    }

    return tx.Commit(ctx)
}
```

### User Registration

```go
func registerUser(ctx context.Context, db *runtime.DB, user User) error {
    tx, _ := db.Begin(ctx)
    defer tx.Rollback(ctx)

    // Create user
    _, err := tx.Exec(ctx, "INSERT INTO users (...) VALUES (...)")
    if err != nil {
        return err
    }

    // Create profile
    _, err = tx.Exec(ctx, "INSERT INTO profiles (...) VALUES (...)")
    if err != nil {
        return err
    }

    // Send welcome email (idempotent!)
    // ...

    return tx.Commit(ctx)
}
```

## Transaction Best Practices

### ✅ DO:

- Always use `defer tx.Rollback(ctx)` immediately after Begin
- Keep transactions short-lived
- Handle all errors properly
- Use for operations that must be atomic
- Test rollback behavior

### ❌ DON'T:

- Forget to defer rollback
- Leave transactions open for too long
- Mix transactional and non-transactional code
- Ignore transaction errors
- Nest transactions naively

## Error Handling Patterns

### Pattern 1: Early Return

```go
tx, _ := db.Begin(ctx)
defer tx.Rollback(ctx)

if err := step1(tx, ctx); err != nil {
    return err  // Rollback via defer
}

if err := step2(tx, ctx); err != nil {
    return err  // Rollback via defer
}

return tx.Commit(ctx)
```

### Pattern 2: Named Return

```go
func doWork(ctx context.Context, db *runtime.DB) (err error) {
    tx, _ := db.Begin(ctx)
    defer func() {
        if err != nil {
            tx.Rollback(ctx)
        }
    }()

    // Operations...

    err = tx.Commit(ctx)
    return
}
```

### Pattern 3: Explicit Rollback

```go
tx, _ := db.Begin(ctx)

err := performOperation(tx, ctx)
if err != nil {
    tx.Rollback(ctx)
    return err
}

return tx.Commit(ctx)
```

## Isolation Levels

PostgreSQL supports different isolation levels:

```go
// Default: Read Committed
tx, _ := db.Begin(ctx)

// Serializable (strictest)
tx, _ := db.BeginTx(ctx, pgx.TxOptions{
    IsoLevel: pgx.Serializable,
})

// Repeatable Read
tx, _ := db.BeginTx(ctx, pgx.TxOptions{
    IsoLevel: pgx.RepeatableRead,
})
```

### Isolation Levels Explained

| Level                | Dirty Read      | Non-Repeatable Read | Phantom Read    |
| -------------------- | --------------- | ------------------- | --------------- |
| **Read Uncommitted** | ✅ Possible     | ✅ Possible         | ✅ Possible     |
| **Read Committed**   | ❌ Not Possible | ✅ Possible         | ✅ Possible     |
| **Repeatable Read**  | ❌ Not Possible | ❌ Not Possible     | ✅ Possible     |
| **Serializable**     | ❌ Not Possible | ❌ Not Possible     | ❌ Not Possible |

**PostgreSQL default: Read Committed**

## Performance Tips

### Keep Transactions Short

```go
// ❌ Bad: Long-running transaction
tx, _ := db.Begin(ctx)
defer tx.Rollback(ctx)

// Lots of work...
time.Sleep(10 * time.Second)
 // Blocks other transactions!

tx.Commit(ctx)
```

```go
// ✅ Good: Short transaction
// Do non-DB work first
email := prepareEmail(user)

// Quick transaction
tx, _ := db.Begin(ctx)
defer tx.Rollback(ctx)

_, err := tx.Exec(ctx, "INSERT ...")
tx.Commit(ctx)

// Send email after transaction
sendEmail(email)
```

### Retry on Conflicts

```go
func withRetry(ctx context.Context, db *runtime.DB, fn func(tx pgx.Tx) error) error {
    maxRetries := 3

    for i := 0; i < maxRetries; i++ {
        tx, _ := db.Begin(ctx)

        err := fn(tx)
        if err == nil {
            return tx.Commit(ctx)
        }

        tx.Rollback(ctx)

        // Check if serialization error
        if isSerializationError(err) {
            continue  // Retry
        }

        return err  // Other error
    }

    return errors.New("max retries exceeded")
}
```

## Deadlock Prevention

### Tips to avoid deadlocks:

1. **Access tables in consistent order**

```go
// ✅ Always: users → accounts → orders
tx.Exec(ctx, "UPDATE users ...")
tx.Exec(ctx, "UPDATE accounts ...")
tx.Exec(ctx, "UPDATE orders ...")
```

2. **Use shorter transactions**
3. **Lock rows explicitly if needed**

```sql
SELECT * FROM accounts WHERE id = 1 FOR UPDATE;
```

4. **Use appropriate isolation levels**

## Testing Transactions

### Test Rollback Behavior

```go
func TestTransactionRollback(t *testing.T) {
    tx, _ := db.Begin(ctx)
    defer tx.Rollback(ctx)

    // Make changes
    tx.Exec(ctx, "INSERT INTO accounts ...")

    // Verify in transaction
    var count int
    tx.QueryRow(ctx, "SELECT COUNT(*) FROM accounts").Scan(&count)
    assert.Equal(t, 1, count)

    // Rollback
    tx.Rollback(ctx)

    // Verify rolled back
    db.QueryRow(ctx, "SELECT COUNT(*) FROM accounts").Scan(&count)
    assert.Equal(t, 0, count)
}
```

## Learn More

- **PostgreSQL Transactions**: https://www.postgresql.org/docs/current/tutorial-transactions.html
- **Isolation Levels**: https://www.postgresql.org/docs/current/transaction-iso.html
- **pgx Documentation**: https://pkg.go.dev/github.com/jackc/pgx/v5

## Key Takeaways

1. **Atomicity** - All operations succeed or all fail
2. **defer Rollback** - Always use as safety net
3. **Short Transactions** - Minimize lock duration
4. **Error Handling** - Proper error checking is critical
5. **Production Pattern** - Same pattern used in banking, e-commerce, etc.

**This example shows how to handle money transfers and critical operations safely!** 💰
