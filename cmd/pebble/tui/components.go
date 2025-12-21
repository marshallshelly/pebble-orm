package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmationDialog represents a yes/no confirmation dialog
type ConfirmationDialog struct {
	Title       string
	Message     string
	YesSelected bool
	OnConfirm   func() tea.Cmd
	OnCancel    func() tea.Cmd
}

// NewConfirmationDialog creates a new confirmation dialog
func NewConfirmationDialog(title, message string) ConfirmationDialog {
	return ConfirmationDialog{
		Title:       title,
		Message:     message,
		YesSelected: false,
	}
}

// Update handles confirmation dialog updates
func (d *ConfirmationDialog) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			d.YesSelected = true
			return nil
		case "right", "l":
			d.YesSelected = false
			return nil
		case "enter":
			if d.YesSelected && d.OnConfirm != nil {
				return d.OnConfirm()
			}
			if !d.YesSelected && d.OnCancel != nil {
				return d.OnCancel()
			}
			return nil
		}
	}
	return nil
}

// View renders the confirmation dialog
func (d ConfirmationDialog) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(d.Title))
	b.WriteString("\n\n")
	b.WriteString(d.Message)
	b.WriteString("\n\n")

	yesButton := inactiveButtonStyle.Render("Yes")
	noButton := inactiveButtonStyle.Render("No")

	if d.YesSelected {
		yesButton = activeButtonStyle.Render("Yes")
	} else {
		noButton = activeButtonStyle.Render("No")
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, yesButton, "  ", noButton))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render(FormatKey("←/→", "navigate") + " • " + FormatKey("enter", "confirm") + " • " + FormatKey("esc/q", "cancel")))

	return boxStyle.Render(b.String())
}

// MigrationItem represents a migration in the list
type MigrationItem struct {
	Version   string
	Name      string
	Status    string
	AppliedAt string
}

func (i MigrationItem) FilterValue() string { return i.Name }
func (i MigrationItem) Title() string {
	statusIcon := FormatStatus(i.Status)
	return fmt.Sprintf("%s %s - %s", statusIcon, i.Version, i.Name)
}
func (i MigrationItem) Description() string {
	if i.AppliedAt != "" {
		return mutedStyle.Render("Applied: " + i.AppliedAt)
	}
	return mutedStyle.Render("Not applied")
}

// MigrationItemDelegate is a custom delegate for migration list items
type MigrationItemDelegate struct{}

func (d MigrationItemDelegate) Height() int                             { return 2 }
func (d MigrationItemDelegate) Spacing() int                            { return 1 }
func (d MigrationItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d MigrationItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(MigrationItem)
	if !ok {
		return
	}

	var s string
	if index == m.Index() {
		s = selectedItemStyle.Render("▸ " + i.Title() + "\n  " + i.Description())
	} else {
		s = unselectedItemStyle.Render("  " + i.Title() + "\n  " + i.Description())
	}

	_, _ = fmt.Fprint(w, s)
}

// ProgressView represents a progress indicator
type ProgressView struct {
	Current int
	Total   int
	Message string
}

// View renders the progress view
func (p ProgressView) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Migration Progress"))
	b.WriteString("\n\n")

	if p.Message != "" {
		b.WriteString(infoStyle.Render(p.Message))
		b.WriteString("\n\n")
	}

	bar := FormatProgressBar(p.Current, p.Total, 40)
	b.WriteString(bar)

	return boxStyle.Render(b.String())
}

// LogView displays migration logs
type LogView struct {
	Logs   []string
	MaxLen int
}

// NewLogView creates a new log view
func NewLogView(maxLen int) LogView {
	return LogView{
		Logs:   make([]string, 0),
		MaxLen: maxLen,
	}
}

// AddLog adds a log entry
func (l *LogView) AddLog(entry string) {
	l.Logs = append(l.Logs, entry)
	if len(l.Logs) > l.MaxLen {
		l.Logs = l.Logs[1:]
	}
}

// View renders the log view
func (l LogView) View() string {
	if len(l.Logs) == 0 {
		return mutedStyle.Render("No logs")
	}

	var b strings.Builder
	for _, log := range l.Logs {
		b.WriteString(mutedStyle.Render("• "))
		b.WriteString(log)
		b.WriteString("\n")
	}

	return boxStyle.Render(b.String())
}
