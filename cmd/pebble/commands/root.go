package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	dbURL         string
	migrationsDir string
	verbose       bool
	jsonOutput    bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "pebble",
	Short: "Pebble ORM - Type-safe PostgreSQL ORM for Go",
	Long: `Pebble ORM is a type-safe PostgreSQL ORM for Go with struct-tag based schemas,
a generic query builder, and comprehensive migration system.

Features:
  - Type-safe query builder with generics
  - Automatic migration generation from struct tags
  - Database introspection and schema diffing
  - Interactive TUI and non-interactive CLI modes
  - Full transaction support with savepoints
  - Relationship loading (belongsTo, hasOne, hasMany, manyToMany)`,
	Version: "1.16.3",
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&dbURL, "db", "", "Database connection URL (required for most commands)")
	rootCmd.PersistentFlags().StringVar(&migrationsDir, "migrations-dir", "./migrations", "Directory for migration files")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}
