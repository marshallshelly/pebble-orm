package output

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

var (
	// Color styles for terminal output
	colorSuccess = lipgloss.Color("#10B981")
	colorWarning = lipgloss.Color("#F59E0B")
	colorError   = lipgloss.Color("#EF4444")
	colorInfo    = lipgloss.Color("#3B82F6")
	colorMuted   = lipgloss.Color("#6B7280")
	colorPrimary = lipgloss.Color("#7C3AED")

	successStyle = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(colorInfo)
	mutedStyle   = lipgloss.NewStyle().Foreground(colorMuted)
	primaryStyle = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
)

// paint renders text for stdout, reducing colour to what stdout supports and
// removing it when stdout isn't a terminal.
func paint(style lipgloss.Style, text string) string {
	return lipgloss.Sprint(style.Render(text))
}

// Success prints a success message
func Success(format string, args ...any) {
	fmt.Print(paint(successStyle, "✓ "))
	fmt.Printf(format+"\n", args...)
}

// Warning prints a warning message
func Warning(format string, args ...any) {
	fmt.Print(paint(warningStyle, "⚠ "))
	fmt.Printf(format+"\n", args...)
}

// Error prints an error message
func Error(format string, args ...any) {
	fmt.Print(paint(errorStyle, "✗ "))
	fmt.Printf(format+"\n", args...)
}

// Info prints an info message
func Info(format string, args ...any) {
	fmt.Print(paint(infoStyle, "ℹ "))
	fmt.Printf(format+"\n", args...)
}

// Muted prints a muted message
func Muted(format string, args ...any) {
	fmt.Print(paint(mutedStyle, fmt.Sprintf(format, args...)))
	fmt.Println()
}

// Primary prints a primary message
func Primary(format string, args ...any) {
	fmt.Print(paint(primaryStyle, fmt.Sprintf(format, args...)))
	fmt.Println()
}

// Section prints a section header
func Section(title string) {
	fmt.Println()
	fmt.Println(paint(primaryStyle, title))
	fmt.Println(paint(mutedStyle, lipgloss.NewStyle().Width(len(title)).Render("═"+lipgloss.NewStyle().Width(len(title)-1).Render("═"))))
	fmt.Println()
}

// StatusIcon returns a colored status icon
func StatusIcon(status string) string {
	switch status {
	case "applied":
		return paint(successStyle, "✓")
	case "pending":
		return paint(warningStyle, "○")
	case "failed":
		return paint(errorStyle, "✗")
	case "running":
		return paint(infoStyle, "◉")
	default:
		return paint(mutedStyle, "•")
	}
}
