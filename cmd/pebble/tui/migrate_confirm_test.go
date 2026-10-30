package tui

import (
	"fmt"
	"image/color"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/marshallshelly/pebble-orm/pkg/migration"
)

// The confirmation dialog's callbacks are closures over a value-type model, so
// anything they assign to m is discarded when Update returns the real model.
// They must communicate by message instead. These tests need no terminal and no
// database: a Bubbletea model is just a function over messages.

func TestConfirmSendsStartedMessageWithTheRealTotal(t *testing.T) {
	m := NewMigrateModel("up", "postgres://unused", "/nonexistent")
	m.migrations = []migration.Migration{{Version: "20260101120000", Name: "first"}}
	m.status = []migration.MigrationRecord{{
		Version: "20260101120000", Name: "first", Status: migration.StatusPending,
	}}

	// enter on a pending migration opens the dialog
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(MigrateModel)
	if m.mode != ModeConfirm {
		t.Fatalf("mode = %v, want ModeConfirm", m.mode)
	}
	if m.confirmation.OnConfirm == nil {
		t.Fatal("OnConfirm not set after opening the dialog")
	}

	cmd := m.confirmation.OnConfirm()
	if cmd == nil {
		t.Fatal("OnConfirm returned no command")
	}
	msg := cmd()
	started, ok := msg.(migrationStartedMsg)
	if !ok {
		t.Fatalf("OnConfirm produced %T, want migrationStartedMsg", msg)
	}
	if started.total != 1 {
		t.Errorf("started.total = %d, want 1 (a closure writing m.progress would leave it 0)", started.total)
	}
	if started.message == "" {
		t.Error("started.message is empty; the progress view would show no label")
	}
}

func TestCancelReturnsToTheList(t *testing.T) {
	m := NewMigrateModel("up", "postgres://unused", "/nonexistent")
	m.mode = ModeConfirm
	m.selectedItems = []int{0}

	next, _ := m.Update(migrationCancelledMsg{})
	m = next.(MigrateModel)

	if m.mode != ModeList {
		t.Errorf("mode = %v, want ModeList: choosing No must dismiss the dialog", m.mode)
	}
	if m.selectedItems != nil {
		t.Errorf("selectedItems = %v, want nil after cancelling", m.selectedItems)
	}
}

func TestSpaceOpensTheDialog(t *testing.T) {
	m := NewMigrateModel("up", "postgres://unused", "/nonexistent")
	m.migrations = []migration.Migration{{Version: "20260101120000", Name: "first"}}
	m.status = []migration.MigrationRecord{{
		Version: "20260101120000", Name: "first", Status: migration.StatusPending,
	}}

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if mode := next.(MigrateModel).mode; mode != ModeConfirm {
		t.Errorf("mode = %v, want ModeConfirm", mode)
	}
}

func TestListQuitKeys(t *testing.T) {
	m := NewMigrateModel("up", "postgres://unused", "/nonexistent")
	if keys := m.list.KeyMap.Quit.Keys(); !slices.Equal(keys, []string{"q", "esc"}) {
		t.Errorf("list quit keys = %v, want q and esc", keys)
	}
	if help := m.list.KeyMap.Quit.Help(); help.Desc != "quit" {
		t.Errorf("list quit help = %q", help.Desc)
	}
}

func TestViewUsesTheAltScreen(t *testing.T) {
	m := NewMigrateModel("up", "postgres://unused", "/nonexistent")
	if !m.View().AltScreen {
		t.Error("the migration UI must run on the alternate screen")
	}
}

func TestBackgroundColorSelectsListStyles(t *testing.T) {
	m := NewMigrateModel("up", "postgres://unused", "/nonexistent")
	dark := m.list.Styles.NoItems.GetForeground()

	next, _ := m.Update(tea.BackgroundColorMsg{Color: color.White})
	m = next.(MigrateModel)

	if m.list.Styles.NoItems.GetForeground() == dark {
		t.Error("a light background must switch the list to its light styles")
	}
	if m.list.Styles.Title.GetForeground() != titleStyle.GetForeground() {
		t.Error("the list title keeps its own style")
	}
}

func TestEnterWithNothingToSelect(t *testing.T) {
	m := NewMigrateModel("up", "postgres://unused", "/nonexistent")

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if mode := next.(MigrateModel).mode; mode != ModeList {
		t.Errorf("before migrations load: mode = %v, want ModeList", mode)
	}

	next, _ = m.Update(migrationsLoadedMsg{})
	next, _ = next.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if mode := next.(MigrateModel).mode; mode != ModeList {
		t.Errorf("with no migrations: mode = %v, want ModeList", mode)
	}
}

// A filtered list numbers its visible rows from zero, so the row under the
// cursor has to be mapped back to its migration.
func TestEnterOnAFilteredListSelectsThatMigration(t *testing.T) {
	var next tea.Model = NewMigrateModel("up", "postgres://unused", "/nonexistent")
	next, _ = next.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	migrations := []migration.Migration{
		{Version: "1", Name: "users"}, {Version: "2", Name: "posts"}, {Version: "3", Name: "tags"},
	}
	var status []migration.MigrationRecord
	for _, mig := range migrations {
		status = append(status, migration.MigrationRecord{Version: mig.Version, Name: mig.Name, Status: migration.StatusPending})
	}
	next, _ = next.Update(migrationsLoadedMsg{migrations: migrations, status: status})

	m := next.(MigrateModel)
	m.list.SetFilterText("tags")
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	m = next.(MigrateModel)
	if !slices.Equal(m.selectedItems, []int{2}) {
		t.Fatalf("selectedItems = %v, want the tags migration at index 2", m.selectedItems)
	}
	if !strings.Contains(m.confirmation.Message, "tags") {
		t.Errorf("dialog names the wrong migration: %q", m.confirmation.Message)
	}
	msg := m.confirmation.OnConfirm()()
	if started := msg.(migrationStartedMsg); !strings.Contains(started.message, "tags") {
		t.Errorf("started %q", started.message)
	}
}

func filteringModel(t *testing.T, names ...string) MigrateModel {
	t.Helper()
	var next tea.Model = NewMigrateModel("up", "postgres://unused", "/nonexistent")
	next, _ = next.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	var migrations []migration.Migration
	var status []migration.MigrationRecord
	for i, name := range names {
		version := fmt.Sprint(i + 1)
		migrations = append(migrations, migration.Migration{Version: version, Name: name})
		status = append(status, migration.MigrationRecord{Version: version, Name: name, Status: migration.StatusPending})
	}
	next, _ = next.Update(migrationsLoadedMsg{migrations: migrations, status: status})
	next, _ = next.Update(tea.KeyPressMsg{Code: '/', Text: "/"})

	m := next.(MigrateModel)
	if !m.list.SettingFilter() {
		t.Fatal("/ should start the filter")
	}
	return m
}

func TestTypingInTheFilterStaysInTheFilter(t *testing.T) {
	m := filteringModel(t, "users", "quotes")

	for _, ch := range []rune{'q', ' ', 'h'} {
		next, cmd := m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Fatalf("typing %q in the filter quit the UI", ch)
			}
		}
		m = next.(MigrateModel)
		if m.mode != ModeList {
			t.Fatalf("typing %q left the list: mode = %v", ch, m.mode)
		}
	}
	if got := m.list.FilterInput.Value(); got != "q h" {
		t.Errorf("filter text = %q, want %q", got, "q h")
	}
}

func TestEnterFinishesTheFilterInsteadOfSelecting(t *testing.T) {
	m := filteringModel(t, "users", "quotes")
	next, _ := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	next, _ = next.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	m = next.(MigrateModel)
	if m.mode != ModeList {
		t.Errorf("enter opened mode %v, want the filter accepted first", m.mode)
	}
	if m.list.SettingFilter() {
		t.Error("enter should finish typing the filter")
	}
}
