# Custom Table Names Example

This example demonstrates how to use comment directives to specify custom table names in Pebble ORM.

## Feature

Instead of relying on automatic `snake_case` conversion of struct names, you can now specify custom table names using a comment directive:

```go
// table_name: custom_users_table
type User struct {
    ID    int    `po:"id,primaryKey,serial"`
    Name  string `po:"name,varchar(100),notNull"`
    Email string `po:"email,varchar(255),unique,notNull"`
}
```

## Comment Directive Format

```go
// table_name: your_custom_name
type YourStruct struct {
    // ...
}
```

**Rules:**

- Must be a line comment (`//`) directly above the struct declaration
- Format: `// table_name: table_name_value`
- Whitespace around the colon is optional
- Table name must be a valid PostgreSQL identifier (alphanumeric + underscores)

## Examples

### Custom Table Name

```go
// table_name: custom_users_table
type User struct {
    ID int `po:"id,primaryKey,serial"`
}
// Table name in database: "custom_users_table"
```

### Default Snake Case

```go
type UserProfile struct {
    ID int `po:"id,primaryKey,serial"`
}
// Table name in database: "user_profile" (automatic conversion)
```

### Multiple Directives

```go
// This is a regular comment
// table_name: products_inventory
// Another comment
type Product struct {
    ID int `po:"id,primaryKey,serial"`
}
// Table name in database: "products_inventory"
```

## Use Cases

1. **Legacy Database Compatibility**: Map to existing tables with non-standard names
2. **Plural Table Names**: Use `users` instead of `user`
3. **Custom Naming Conventions**: Follow your team's specific naming standards
4. **Complex Names**: Use names that don't match Go struct naming conventions

## How It Works

1. **Source File Parsing**: Pebble ORM parses the Go source file at registration time
2. **Comment Extraction**: Looks for `table_name:` directive in comments above struct
3. **Fallback**: If no directive is found, uses default `snake_case` conversion

## Running the Example

```bash
cd examples/custom_table_names
go run main.go
```

## Important Notes

- The comment directive is read when you call `registry.Register(Model{})`
- Source file must be accessible (works well in development and with Go modules)
- If source file cannot be found, falls back to `snake_case` conversion
- Works with both single-file and multi-file packages

## Migration Generation

When generating migrations, the custom table names will be used:

```bash
pebble generate --name add_custom_tables --db "postgres://..."
```

The generated migration will create tables with your custom names!
