package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// An autoIncrement column with a plain integer type must map to the matching
// serial type so PostgreSQL creates a backing sequence.
func TestAutoIncrementMapsToSerial(t *testing.T) {
	p := NewPlanner()
	cases := map[string]string{
		"bigint":   "bigserial",
		"integer":  "serial",
		"smallint": "smallserial",
	}
	for in, want := range cases {
		def := p.generateColumnDefinition(schema.ColumnMetadata{
			Name: "id", SQLType: in, AutoIncrement: true, Nullable: false,
		})
		if !strings.Contains(def, want) {
			t.Errorf("autoIncrement %s: expected %q in %q", in, want, def)
		}
	}
	// An explicit identity column is not rewritten to serial.
	def := p.generateColumnDefinition(schema.ColumnMetadata{
		Name: "id", SQLType: "bigint", AutoIncrement: true,
		Identity: &schema.IdentityColumn{Generation: schema.IdentityAlways},
	})
	if strings.Contains(def, "serial") {
		t.Errorf("identity column should not become serial: %q", def)
	}
}

// A migration containing CREATE/DROP INDEX CONCURRENTLY must be detected so it is
// applied outside a transaction block.
func TestHasNonTransactionalStmt(t *testing.T) {
	yes := []string{"CREATE INDEX CONCURRENTLY idx_a ON t (c);"}
	if !hasNonTransactionalStmt(yes) {
		t.Error("CREATE INDEX CONCURRENTLY should be detected as non-transactional")
	}
	no := []string{"CREATE TABLE t (id serial);", "CREATE INDEX idx_a ON t (c);"}
	if hasNonTransactionalStmt(no) {
		t.Error("plain statements should not be flagged non-transactional")
	}
}
