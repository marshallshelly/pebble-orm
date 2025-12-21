package output

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
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

// Success prints a success message
func Success(format string, args ...interface{}) {
	fmt.Print(successStyle.Render("✓ "))
	fmt.Printf(format+"\n", args...)
}

// Warning prints a warning message
func Warning(format string, args ...interface{}) {
	fmt.Print(warningStyle.Render("⚠ "))
	fmt.Printf(format+"\n", args...)
}

// Error prints an error message
func Error(format string, args ...interface{}) {
	fmt.Print(errorStyle.Render("✗ "))
	fmt.Printf(format+"\n", args...)
}

// Info prints an info message
func Info(format string, args ...interface{}) {
	fmt.Print(infoStyle.Render("ℹ "))
	fmt.Printf(format+"\n", args...)
}

// Muted prints a muted message
func Muted(format string, args ...interface{}) {
	fmt.Print(mutedStyle.Render(fmt.Sprintf(format, args...)))
	fmt.Println()
}

// Primary prints a primary message
func Primary(format string, args ...interface{}) {
	fmt.Print(primaryStyle.Render(fmt.Sprintf(format, args...)))
	fmt.Println()
}

// Section prints a section header
func Section(title string) {
	fmt.Println()
	fmt.Println(primaryStyle.Render(title))
	fmt.Println(mutedStyle.Render(lipgloss.NewStyle().Width(len(title)).Render("═" + lipgloss.NewStyle().Width(len(title)-1).Render("═"))))
	fmt.Println()
}

// StatusIcon returns a colored status icon
func StatusIcon(status string) string {
	switch status {
	case "applied":
		return successStyle.Render("✓")
	case "pending":
		return warningStyle.Render("○")
	case "failed":
		return errorStyle.Render("✗")
	case "running":
		return infoStyle.Render("◉")
	default:
		return mutedStyle.Render("•")
	}
}
