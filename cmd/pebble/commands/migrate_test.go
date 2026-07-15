package commands

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/migration"
)

func TestSelectPendingMigrations(t *testing.T) {
	migs := []migration.Migration{
		{Version: "001"}, {Version: "002"}, {Version: "003"},
	}

	tests := []struct {
		name    string
		applied map[string]bool
		all     bool
		steps   int
		want    []string
	}{
		{"all on empty db", nil, true, 0, []string{"001", "002", "003"}},
		{"all on partially applied db skips applied", map[string]bool{"001": true}, true, 0, []string{"002", "003"}},
		{"all when fully applied", map[string]bool{"001": true, "002": true, "003": true}, true, 0, nil},
		{"steps 1 from empty", nil, false, 1, []string{"001"}},
		{"steps 2 skips first applied", map[string]bool{"001": true}, false, 2, []string{"002", "003"}},
		{"steps beyond remaining", map[string]bool{"001": true, "002": true}, false, 5, []string{"003"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectPendingMigrations(migs, tt.applied, tt.all, tt.steps)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d migrations %v, want %d %v", len(got), versions(got), len(tt.want), tt.want)
			}
			for i, v := range tt.want {
				if got[i].Version != v {
					t.Errorf("index %d: got %s, want %s", i, got[i].Version, v)
				}
			}
		})
	}
}

func versions(m []migration.Migration) []string {
	out := make([]string, len(m))
	for i := range m {
		out[i] = m[i].Version
	}
	return out
}
