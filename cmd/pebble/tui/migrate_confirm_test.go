package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
