// tui/dashboard/resource.go
package dashboard

import (
	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// Column defines a table column with title and width.
type Column struct {
	Title string
	Width int
}

// Action is an operation that can be performed on a selected resource.
type Action struct {
	Name string
	Run  func() tea.Cmd
}

// Resource defines a browsable resource type for the dashboard.
type Resource interface {
	// Name returns the display name (e.g. "EC2 Instances").
	Name() string

	// Columns returns the table column definitions.
	Columns() []Column

	// FetchCmd returns a tea.Cmd that fetches resource data.
	FetchCmd(client *bargeaws.Client) tea.Cmd

	// HandleMsg processes a fetch result message. Returns true if handled.
	HandleMsg(msg tea.Msg) bool

	// Rows returns the current data as table rows.
	Rows() []table.Row

	// Actions returns available actions for the given selected row.
	Actions(row table.Row) []Action

	// Error returns any error from the last fetch.
	Error() error
}
