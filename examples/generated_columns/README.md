# Generated Columns Example

This example demonstrates **PostgreSQL generated columns** in Pebble ORM.

## What are Generated Columns?

Generated columns are columns whose values are automatically computed from other columns in the same table. PostgreSQL supports `STORED` generated columns (computed on INSERT/UPDATE and stored in the table).

## Features Demonstrated

### 1. STORED Generated Columns

```go
type Person struct {
    FirstName string  `po:"first_name"`
    LastName  string  `po:"last_name"`
    // Automatically computed from first_name and last_name
    FullName  string  `po:"full_name,generated:first_name || ' ' || last_name,stored"`
}
```

### 2. Numeric Calculations

```go
type Measurement struct {
    HeightCm float64 `po:"height_cm"`
    // Automatically convert cm to inches
    HeightIn float64 `po:"height_in,generated:height_cm / 2.54,stored"`
}
```

### 3. Complex Expressions

```go
type Product struct {
    Price    float64 `po:"price"`
    TaxRate  float64 `po:"tax_rate"`
    // Calculate price with tax
    PriceWithTax float64 `po:"price_with_tax,generated:price * tax_rate,stored"`
}
```

## Tag Syntax

```go
`po:"column_name,generated:EXPRESSION,stored"`
```

- **`generated:EXPRESSION`**: SQL expression to compute the value
- **`stored`**: Store the computed value (default if omitted)
- **`virtual`**: Compute on read (reserved for future PostgreSQL support)

## Generated SQL

```sql
CREATE TABLE IF NOT EXISTS people (
    first_name varchar(255),
    last_name varchar(255),
    full_name varchar(255) GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED
);
```

## Benefits

✅ **Data Consistency**: Values are always computed correctly  
✅ **No Application Logic**: Database handles the computation  
✅ **Indexed**: Generated columns can be indexed for fast queries  
✅ **Automatic Updates**: Changes to source columns update generated columns

## Limitations

Generated columns have specific constraints in PostgreSQL:

### **Column Constraints:**

- ❌ Cannot have `NOT NULL` constraint (use expression logic instead)
- ❌ Cannot have `DEFAULT` value
- ❌ Cannot have `IDENTITY` definition
- ❌ Cannot have `UNIQUE` constraint directly (create separate index instead)
- ✅ Can be indexed (create index separately)

### **Expression Constraints:**

- ❌ Cannot use subqueries
- ❌ Cannot reference other generated columns
- ❌ Cannot use volatile functions (`CURRENT_TIMESTAMP`, `RANDOM()`, etc.)
- ❌ Cannot reference system columns (except `tableoid`)
- ✅ Must use immutable functions only

### **Table Constraints:**

- ❌ Cannot be part of a partition key
- ❌ Cannot be used in `BEFORE` triggers
- ✅ Read-only - cannot INSERT or UPDATE directly

### **Workarounds:**

**For UNIQUE constraint:**

```sql
-- Instead of: column GENERATED ... UNIQUE
-- Do this:
CREATE UNIQUE INDEX idx_unique_column ON table_name (generated_column);
```

**For NOT NULL:**

```go
// Ensure source columns are NOT NULL instead
type Person struct {
    FirstName string `po:"first_name,notNull"`  // ✅ Source is NOT NULL
    LastName  string `po:"last_name,notNull"`   // ✅ Source is NOT NULL
    FullName  string `po:"full_name,generated:first_name || ' ' || last_name,stored"`
}
```

## Example Usage

```go
package main

import (
    "context"
    "log"

    "github.com/marshallshelly/pebble-orm/pkg/builder"
    "github.com/marshallshelly/pebble-orm/pkg/registry"
    "github.com/marshallshelly/pebble-orm/pkg/runtime"
)

type Person struct {
    ID        int64  `po:"id,primaryKey,autoIncrement"`
    FirstName string `po:"first_name"`
    LastName  string `po:"last_name"`
    FullName  string `po:"full_name,generated:first_name || ' ' || last_name,stored"`
}

func main() {
    ctx := context.Background()

    // Connect to database
    db, _ := runtime.Connect(ctx, runtime.Config{
        Host:     "localhost",
        Port:     5432,
        Database: "mydb",
        User:     "postgres",
        Password: "password",
    })
    defer db.Close()

    // Register model
    registry.Register(Person{})

    // Run migrations (creates table with generated column)
    // ... migration code ...

    // Insert a person (only provide first_name and last_name)
    qb := builder.New(db)
    person := Person{
        FirstName: "John",
        LastName:  "Doe",
        // FullName is automatically computed!
    }

    result, _ := builder.Insert[Person](qb).
        Values(person).
        Returning("*").
        ExecReturning(ctx)

    log.Printf("Full name: %s", result[0].FullName)
    // Output: Full name: John Doe
}
```

## Common Use Cases

### 1. Full Names

```go
FullName string `po:"full_name,generated:first_name || ' ' || last_name,stored"`
```

### 2. Unit Conversions

```go
HeightIn float64 `po:"height_in,generated:height_cm / 2.54,stored"`
TempF    float64 `po:"temp_f,generated:temp_c * 9 / 5 + 32,stored"`
```

### 3. Calculations

```go
Total    float64 `po:"total,generated:price * quantity,stored"`
Discount float64 `po:"discount,generated:price * discount_rate,stored"`
```

### 4. String Formatting

```go
Email string `po:"email,generated:LOWER(username || '@example.com'),stored"`
Slug  string `po:"slug,generated:LOWER(REPLACE(title, ' ', '-')),stored"`
```

## PostgreSQL Documentation

For more information, see:

- [PostgreSQL Generated Columns](https://www.postgresql.org/docs/current/ddl-generated-columns.html)

## Key Takeaways

1. **Automatic Computation**: Database computes values automatically
2. **Type-Safe**: Defined in Go structs with `po` tags
3. **Migration Support**: Pebble ORM generates correct DDL
4. **Read-Only**: Cannot be manually set in INSERT/UPDATE
5. **Indexable**: Can create indexes on generated columns

**Generated columns keep your data consistent and reduce application complexity!** 🎉
