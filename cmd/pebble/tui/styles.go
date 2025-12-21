package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Color palette
	colorPrimary   = lipgloss.Color("#7C3AED")
	colorSecondary = lipgloss.Color("#EC4899")
	colorSuccess   = lipgloss.Color("#10B981")
	colorWarning   = lipgloss.Color("#F59E0B")
	colorDanger    = lipgloss.Color("#EF4444")
	colorInfo      = lipgloss.Color("#3B82F6")
	colorMuted     = lipgloss.Color("#6B7280")
	colorText      = lipgloss.Color("#F3F4F6")
	colorBorder    = lipgloss.Color("#4B5563")

	// Base styles
	baseStyle = lipgloss.NewStyle().
			Foreground(colorText)

	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	// Status styles
	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	dangerStyle = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(colorInfo)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// List styles
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true).
				PaddingLeft(2)

	unselectedItemStyle = lipgloss.NewStyle().
				Foreground(colorText).
				PaddingLeft(4)

	// Box styles
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	activeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)

	// Button styles
	activeButtonStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorPrimary).
				Padding(0, 3).
				Bold(true)

	inactiveButtonStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Background(lipgloss.Color("#1F2937")).
				Padding(0, 3)

	// Status indicator styles
	statusAppliedStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				SetString("✓")

	statusPendingStyle = lipgloss.NewStyle().
				Foreground(colorWarning).
				SetString("○")

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(colorDanger).
				SetString("✗")

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(colorInfo).
				SetString("◉")

	// Help styles
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorPrimary)

	// Progress bar styles
	progressBarStyle = lipgloss.NewStyle().
				Foreground(colorPrimary)

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	// Error styles
	errorStyle = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDanger).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	// Code/SQL styles
	codeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Background(lipgloss.Color("#1F2937")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)
)

// FormatStatus returns a styled status indicator
func FormatStatus(status string) string {
	switch status {
	case "applied":
		return statusAppliedStyle.Render() + " " + successStyle.Render(status)
	case "pending":
		return statusPendingStyle.Render() + " " + warningStyle.Render(status)
	case "failed":
		return statusFailedStyle.Render() + " " + dangerStyle.Render(status)
	case "running":
		return statusRunningStyle.Render() + " " + infoStyle.Render(status)
	default:
		return mutedStyle.Render(status)
	}
}

// FormatProgressBar creates a simple progress bar
func FormatProgressBar(current, total int, width int) string {
	if total == 0 {
		return progressEmptyStyle.Render(lipgloss.NewStyle().Width(width).Render(""))
	}

	percentage := float64(current) / float64(total)
	filled := int(float64(width) * percentage)
	empty := width - filled

	bar := progressBarStyle.Render(lipgloss.NewStyle().Width(filled).Render("━")) +
		progressEmptyStyle.Render(lipgloss.NewStyle().Width(empty).Render("━"))

	return bar + " " + infoStyle.Render(lipgloss.NewStyle().Width(10).Align(lipgloss.Right).Render(
		lipgloss.NewStyle().Bold(true).Render(
			lipgloss.NewStyle().Width(3).Align(lipgloss.Right).Render(
				lipgloss.NewStyle().Render(
					fmt.Sprintf("%d/%d", current, total),
				),
			),
		),
	))
}

// FormatKey formats a help key
func FormatKey(key, description string) string {
	return helpKeyStyle.Render(key) + " " + mutedStyle.Render(description)
}
