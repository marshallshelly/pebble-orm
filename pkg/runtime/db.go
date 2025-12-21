package runtime

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB represents a database connection.
type DB struct {
	pool   *pgxpool.Pool
	config *Config
}

// Config represents database configuration.
type Config struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
	MaxConns int32
	MinConns int32
}

// NewDB creates a new DB instance from a connection pool.
func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{
		pool: pool,
		config: &Config{},
	}
}

// Connect creates a new DB instance by connecting to PostgreSQL.
func Connect(ctx context.Context, config *Config) (*DB, error) {
	connString := buildConnectionString(config)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply pool configuration
	if config.MaxConns > 0 {
		poolConfig.MaxConns = config.MaxConns
	}
	if config.MinConns > 0 {
		poolConfig.MinConns = config.MinConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		pool:   pool,
		config: config,
	}, nil
}

// ConnectWithURL creates a new DB instance using a connection URL.
func ConnectWithURL(ctx context.Context, url string) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		pool:   pool,
		config: &Config{},
	}, nil
}

// Pool returns the underlying pgxpool.Pool.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// Close closes the database connection pool.
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// Ping verifies the database connection is alive.
func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// Begin starts a new transaction.
func (db *DB) Begin(ctx context.Context) (pgx.Tx, error) {
	return db.pool.Begin(ctx)
}

// BeginTx starts a new transaction with options.
func (db *DB) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return db.pool.BeginTx(ctx, txOptions)
}

// Exec executes a query without returning any rows.
func (db *DB) Exec(ctx context.Context, sql string, args ...interface{}) (int64, error) {
	result, err := db.pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, &QueryError{Query: sql, Err: err}
	}
	return result.RowsAffected(), nil
}

// Query executes a query that returns rows.
func (db *DB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	rows, err := db.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, &QueryError{Query: sql, Err: err}
	}
	return rows, nil
}

// QueryRow executes a query that returns at most one row.
func (db *DB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

// buildConnectionString builds a PostgreSQL connection string from config.
func buildConnectionString(config *Config) string {
	sslMode := config.SSLMode
	if sslMode == "" {
		sslMode = "prefer"
	}

	port := config.Port
	if port == 0 {
		port = 5432
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		port,
		config.User,
		config.Password,
		config.Database,
		sslMode,
	)
}

// DefaultConfig returns a default database configuration.
func DefaultConfig() *Config {
	return &Config{
		Host:     "localhost",
		Port:     5432,
		Database: "postgres",
		User:     "postgres",
		Password: "",
		SSLMode:  "prefer",
		MaxConns: 10,
		MinConns: 2,
	}
}
