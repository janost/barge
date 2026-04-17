// tui/dashboard/resource.go
package dashboard

import (
	"time"

	bridgeaws "github.com/janost/bridge/aws"
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

// Drillable is optionally implemented by Resources that support drill-down.
// When Enter is pressed on a Drillable resource, ChildResource is called
// instead of showing the action menu.
type Drillable interface {
	ChildResource(row table.Row) (label string, child Resource)
}

// SecondaryDrillable is optionally implemented by Drillable resources
// that support an alternate drill-down path (e.g. Shift+Enter).
type SecondaryDrillable interface {
	SecondaryChildResource(row table.Row) (label string, child Resource)
}

// Tailable is optionally implemented by Resources that should auto-refresh
// on their own schedule regardless of the global refresh setting.
type Tailable interface {
	TailInterval() time.Duration
}

// processExitMsg is sent when a subprocess (e.g. SSM shell) completes.
type processExitMsg struct{ err error }

// Resource defines a browsable resource type for the dashboard.
type Resource interface {
	// Name returns the display name (e.g. "EC2 Instances").
	Name() string

	// Columns returns the table column definitions.
	Columns() []Column

	// FetchCmd returns a tea.Cmd that fetches resource data.
	FetchCmd(client *bridgeaws.Client) tea.Cmd

	// HandleMsg processes a fetch result message. Returns true if handled.
	HandleMsg(msg tea.Msg) bool

	// Rows returns the current data as table rows.
	Rows() []table.Row

	// Actions returns available actions for the given selected row.
	Actions(row table.Row) []Action

	// Error returns any error from the last fetch.
	Error() error
}
