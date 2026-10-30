package tui

import (
	"image/color"
	"slices"
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
