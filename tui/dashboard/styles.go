// tui/dashboard/styles.go
package dashboard

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	resourceStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	countStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	shortcutStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	actionCursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	actionNormalStyle = lipgloss.NewStyle().PaddingLeft(2)
)

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("8")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("6")).
		Bold(true)
	return s
}
