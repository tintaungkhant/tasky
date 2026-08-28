// Package ui provides small, consistent styling helpers for tasky's CLI
// output (errors, success, info, warnings), so users can tell message kinds
// apart at a glance. Colors auto-disable on non-TTY output or when NO_COLOR
// is set (handled by lipgloss).
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#B00020", Dark: "#FF6E6E"})
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#4ADE80"})
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0B5FFF", Dark: "#7AA7FF"})
	warnStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#946200", Dark: "#FFC24B"})
	labelStyle   = lipgloss.NewStyle().Bold(true)
)

// Errorf formats an error-styled message (no trailing newline).
func Errorf(format string, args ...any) string {
	return errorStyle.Render(fmt.Sprintf(format, args...))
}

// Success formats a success-styled message, prefixed with a checkmark.
func Success(format string, args ...any) string {
	return successStyle.Render("✓ "+fmt.Sprintf(format, args...))
}

// Info formats an info-styled message.
func Info(format string, args ...any) string {
	return infoStyle.Render(fmt.Sprintf(format, args...))
}

// Warn formats a warning-styled message, prefixed with a warning marker.
func Warn(format string, args ...any) string {
	return warnStyle.Render("! "+fmt.Sprintf(format, args...))
}

// Label styles a field label (e.g. "Task:", "Branch:") for aligned summaries.
func Label(s string) string {
	return labelStyle.Render(s)
}
