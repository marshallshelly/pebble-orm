package commands

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/marshallshelly/pebble-orm/cmd/pebble/output"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
	"github.com/spf13/cobra"
)

var (
	scanDir        string
	metadataOutput string
)

// metadataCmd generates table name metadata from source files
var metadataCmd = &cobra.Command{
	Use:   "metadata",
	Short: "Generate table name metadata from source files",
	Long: `Scan Go source files for // table_name: comments and generate a metadata file.

The generated file registers custom table names at compile-time, making them
available in production builds where source files don't exist.

Examples:
  pebble generate metadata --scan ./internal/models
  pebble generate metadata --scan ./api/models --output ./api/models/table_names.gen.go`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerateMetadata()
	},
}

func init() {
	generateCmd.AddCommand(metadataCmd)

	metadataCmd.Flags().StringVar(&scanDir, "scan", "", "Directory to scan for model definitions (required)")
	metadataCmd.Flags().StringVarP(&metadataOutput, "output", "o", "", "Output file path (default: <scan-dir>/table_names.gen.go)")
	_ = metadataCmd.MarkFlagRequired("scan")
}

func runGenerateMetadata() error {
	// Validate scan directory
	if scanDir == "" {
		return fmt.Errorf("--scan flag is required")
	}

	absPath, err := filepath.Abs(scanDir)
	if err != nil {
		return fmt.Errorf("invalid scan path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("scan directory not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("scan path must be a directory: %s", absPath)
	}

	output.Section("Scanning for table names")
	output.Info("Directory: %s", absPath)

	// Scan for table names
	tableNames, err := scanForTableNames(absPath)
	if err != nil {
		return fmt.Errorf("failed to scan for table names: %w", err)
	}

	if len(tableNames) == 0 {
		output.Warning("No table name directives found in %s", absPath)
		output.Info("Add comments like: // table_name: your_table_name")
		return nil
	}

	output.Success("Found %d table name directive(s)", len(tableNames))
	for structName, tableName := range tableNames {
		fmt.Printf("  %s → %s\n", structName, tableName)
	}
	fmt.Println()

	// Determine output file
	if metadataOutput == "" {
		metadataOutput = filepath.Join(absPath, "table_names.gen.go")
	}

	// Get package name
	pkgName, err := getPackageName(absPath)
	if err != nil {
		return fmt.Errorf("failed to determine package name: %w", err)
	}

	// Generate the file
	if err := generateTableNamesFile(metadataOutput, pkgName, tableNames); err != nil {
		return fmt.Errorf("failed to generate file: %w", err)
	}

	output.Success("Generated: %s", metadataOutput)
	output.Info("Commit this file to version control")
	fmt.Println()
	output.Info("💡 Tip: Run this command before building Docker images to ensure")
	output.Info("   custom table names work in production environments.")

	return nil
}

// scanForTableNames scans a directory for Go files with table_name comments
func scanForTableNames(dir string) (map[string]string, error) {
	tableNames := make(map[string]string)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip generated files
		if strings.HasSuffix(path, ".gen.go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse the file
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip files with parse errors
		}

		// Look for struct declarations with table_name comments
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				// Check if it's a struct
				_, ok = typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				structName := typeSpec.Name.Name

				// Check doc comments
				if genDecl.Doc != nil {
					for _, comment := range genDecl.Doc.List {
						if tableName := schema.ParseTableNameFromComment(comment.Text); tableName != "" {
							tableNames[structName] = tableName
							break
						}
					}
				}
			}
		}

		return nil
	})

	return tableNames, err
}

// getPackageName extracts the package name from the first Go file in a directory
func getPackageName(dir string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}

		if strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(dir, file.Name())
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if err != nil {
			continue
		}

		return f.Name.Name, nil
	}

	return "", fmt.Errorf("no Go files found in %s", dir)
}

// generateTableNamesFile generates the table_names.gen.go file
func generateTableNamesFile(outputPath, packageName string, tableNames map[string]string) error {
	var sb strings.Builder

	sb.WriteString("// Code generated by pebble-orm. DO NOT EDIT.\n")
	sb.WriteString(fmt.Sprintf("// pebble %s\n\n", strings.Join(os.Args[1:], " ")))
	sb.WriteString(fmt.Sprintf("package %s\n\n", packageName))
	sb.WriteString("import \"github.com/marshallshelly/pebble-orm/pkg/schema\"\n\n")
	sb.WriteString("func init() {\n")
	sb.WriteString("\t// Register custom table names from comment directives\n")

	// Sort for consistent output
	for structName, tableName := range tableNames {
		sb.WriteString(fmt.Sprintf("\tschema.RegisterTableName(\"%s\", \"%s\")\n", structName, tableName))
	}

	sb.WriteString("}\n")

	return os.WriteFile(outputPath, []byte(sb.String()), 0644)
}
