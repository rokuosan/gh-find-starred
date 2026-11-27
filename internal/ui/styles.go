package ui

import "charm.land/lipgloss/v2"

// Styles defines the visual styling for the TUI components.
type Styles struct {
	Spinner     lipgloss.Style
	Success     lipgloss.Style
	Error       lipgloss.Style
	Warning     lipgloss.Style
	Muted       lipgloss.Style
	RepoName    lipgloss.Style
	RepoURL     lipgloss.Style
	Description lipgloss.Style
	Score       lipgloss.Style
}

// DefaultStyles returns the default Lip Gloss v2 style palette.
func DefaultStyles() Styles {
	return Styles{
		Spinner:     lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		Success:     lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		Warning:     lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		Muted:       lipgloss.NewStyle().Faint(true),
		RepoName:    lipgloss.NewStyle().Bold(true),
		RepoURL:     lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		Description: lipgloss.NewStyle().Faint(true),
		Score:       lipgloss.NewStyle().Foreground(lipgloss.Color("141")),
	}
}
