# Transactions

<em>Commit, rollback, row locks, savepoints — with the same typed builders you use everywhere else.</em>

Four runnable scenarios against an `accounts` table: a basic commit, a rollback proven by a follow-up count, an atomic money transfer under `FOR UPDATE` locks, and a savepoint that undoes half a transaction while keeping the rest.

## Run

```bash
createdb pebble_transactions
export DATABASE_URL="postgres://localhost:5432/pebble_transactions?sslmode=disable"

cd examples/transactions
pebble generate --name initial_schema --models ./internal/models
pebble migrate up --all --db "$DATABASE_URL"
go run cmd/transactions/main.go
```

## What it shows

- `qb.Begin(ctx)` + `defer tx.Rollback()` as the safety net — rollback is a no-op after a successful `Commit`
- `TxInsert` / `TxSelect` / `TxUpdate` / `TxDelete` — the transaction-scoped builders (note: no `ctx` on their exec methods; the transaction carries it)
- `ForUpdate()` row locking to make the balance check + update race-free
- `Savepoint` / `RollbackToSavepoint` / `ReleaseSavepoint` for partial rollback inside one transaction
- Verification queries after each scenario, so you see the rollback actually happened

## The money transfer

The whole point of transactions in one function: lock both rows, check the balance, apply both updates, commit — or any error unwinds everything via the deferred rollback.

```go
tx, err := qb.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback()

from, err := builder.TxSelect[models.Account](tx).
    Where(builder.Eq("id", fromAccountID)).
    ForUpdate().        // locks the row until commit
    First()
if err != nil {
    return err
}

if from.Balance < transferAmount {
    return fmt.Errorf("insufficient balance")
}

_, err = builder.TxUpdate[models.Account](tx).
    Set("balance", from.Balance-transferAmount).
    Where(builder.Eq("id", fromAccountID)).
    Exec()
// ... mirror update on the destination account ...

return tx.Commit()
```

## Savepoints

Undo part of a transaction without losing the rest:

```go
tx, _ := qb.Begin(ctx)
defer tx.Rollback()

builder.TxInsert[models.Account](tx).Values(account1).ExecReturning() // kept

tx.Savepoint("before_second_account")
builder.TxInsert[models.Account](tx).Values(account2).ExecReturning() // undone
tx.RollbackToSavepoint("before_second_account")

tx.Commit() // account1 persists, account2 never existed
```

`Preload` works on `TxSelect` too — eager loading runs through the same transaction connection.

<details>
<summary><strong>Expected output (excerpt)</strong></summary>

```
--- Example 1: Basic Transaction (Commit) ---
✅ Transaction committed successfully
  - Created account 1 with balance: $1000.00
  - Created account 2 with balance: $500.00

--- Example 2: Transaction Rollback ---
Created account 3 with balance: $10000.00
Simulating error - rolling back transaction...
Accounts with user_id=999 after rollback: 0 (should be 0)

--- Example 3: Money Transfer (Atomic) ---
✅ Successfully transferred $250.00
Final balances:
  From account: $750.00 (deducted $250.00)
  To account: $750.00 (added $250.00)

--- Example 4: Savepoints (Nested Transactions) ---
Created savepoint: before_second_account
Rolled back to savepoint - second account creation undone
  Accounts with user_id=300: 1 (should be 1)
  Accounts with user_id=400: 0 (should be 0)
```

</details>
