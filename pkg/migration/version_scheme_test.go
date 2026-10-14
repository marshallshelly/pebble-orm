package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func writeMigrations(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("-- x\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}
}

func TestNextVersionSequential(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		want     string
	}{
		{"empty dir starts at 000", nil, "000"},
		{"continues from 000", []string{"000_init.up.sql", "000_init.down.sql"}, "001"},
		{"continues from 002", []string{"000_a.up.sql", "001_b.up.sql", "002_c.up.sql"}, "003"},
		{"gaps do not repeat", []string{"000_a.up.sql", "004_b.up.sql"}, "005"},
		{"keeps a wider existing width", []string{"0001_a.up.sql"}, "0002"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMigrations(t, dir, tt.existing...)
			got, err := NextVersion(dir, SequentialVersions)
			if err != nil {
				t.Fatalf("NextVersion() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("NextVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNextVersionRejectsPaddingOverflow(t *testing.T) {
	dir := t.TempDir()
	writeMigrations(t, dir, "999_a.up.sql")

	got, err := NextVersion(dir, SequentialVersions)
	if err == nil {
		t.Errorf("NextVersion() = %q, nil; want an error: %q sorts before \"999\"", got, got)
	}
}

func TestNextVersionDirectorySchemeWins(t *testing.T) {
	dir := t.TempDir()
	writeMigrations(t, dir, "20260101120000_init.up.sql")

	got, err := NextVersion(dir, SequentialVersions)
	if err != nil {
		t.Fatalf("NextVersion() error = %v", err)
	}
	if len(got) != 14 {
		t.Errorf("NextVersion() = %q, want a timestamp: an existing scheme must win over the preference", got)
	}
}

func TestNextVersionDefaultsToTimestamp(t *testing.T) {
	got, err := NextVersion(t.TempDir(), "")
	if err != nil {
		t.Fatalf("NextVersion() error = %v", err)
	}
	if len(got) != 14 {
		t.Errorf("NextVersion() = %q, want a 14-digit timestamp", got)
	}
}

func TestDetectVersionSchemeRejectsMixed(t *testing.T) {
	dir := t.TempDir()
	writeMigrations(t, dir, "000_a.up.sql", "20260101120000_b.up.sql")

	if _, _, err := DetectVersionScheme(dir); err == nil {
		t.Error("DetectVersionScheme() = nil error, want an error for a directory mixing schemes")
	}
}

func TestSequentialFilesSortInApplyOrder(t *testing.T) {
	dir := t.TempDir()
	var got []string
	for range 12 {
		v, err := NextVersion(dir, SequentialVersions)
		if err != nil {
			t.Fatalf("NextVersion() error = %v", err)
		}
		writeMigrations(t, dir, v+"_m.up.sql")
		got = append(got, v)
	}

	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Errorf("version %q does not sort before %q; lexicographic order must match apply order", got[i-1], got[i])
		}
	}
}

// GenerateVersion has one-second resolution, so several generates inside one
// second used to share a version. Both files got written, but migrations are
// keyed by version, so every one after the first became invisible to status and
// never applied.
func TestTimestampVersionsAreUniqueWithinASecond(t *testing.T) {
	dir := t.TempDir()
	g := NewGenerator(dir)

	const n = 5
	versions := make([]string, 0, n)
	for i := range n {
		f, err := g.GenerateEmpty(fmt.Sprintf("m%d", i))
		if err != nil {
			t.Fatalf("GenerateEmpty: %v", err)
		}
		versions = append(versions, f.Version)
	}

	seen := make(map[string]int, n)
	for i, v := range versions {
		if first, dup := seen[v]; dup {
			t.Errorf("migration %d reuses version %s from migration %d", i, v, first)
		}
		seen[v] = i
	}

	// Lexicographic order must equal creation order: reconstruct.go sorts
	// filenames and relies on that being chronological.
	if !slices.IsSorted(versions) {
		t.Errorf("versions are not lexicographically ascending: %v", versions)
	}

	for _, v := range versions {
		if len(v) != 14 {
			t.Errorf("version %q is %d chars, want the 14-digit timestamp format", v, len(v))
		}
	}

	// Every migration written must be visible.
	listed, err := g.ListMigrations()
	if err != nil {
		t.Fatalf("ListMigrations: %v", err)
	}
	if len(listed) != n {
		t.Errorf("ListMigrations returned %d of %d migrations", len(listed), n)
	}
}

// A timestamp that collides with an existing file, not just with a sibling
// created in the same run.
func TestTimestampVersionSkipsAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Format("20060102150405")
	writeMigrations(t, dir, now+"_already_here.up.sql", now+"_already_here.down.sql")

	got, err := NextVersion(dir, TimestampVersions)
	if err != nil {
		t.Fatalf("NextVersion: %v", err)
	}
	if got == now {
		t.Errorf("NextVersion returned %s, which is already taken on disk", got)
	}
	if got <= now {
		t.Errorf("NextVersion returned %s, which does not sort after the existing %s", got, now)
	}
}
