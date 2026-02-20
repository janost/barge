package dashboard

import (
	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type pickerReadyMsg struct{}

// ResourcePicker is the root resource showing available resource types.
type ResourcePicker struct{}

func NewResourcePicker() *ResourcePicker {
	return &ResourcePicker{}
}

func (r *ResourcePicker) Name() string { return "Resources" }

func (r *ResourcePicker) Columns() []Column {
	return []Column{
		{"TYPE", 30},
		{"DESCRIPTION", 50},
	}
}

func (r *ResourcePicker) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg { return pickerReadyMsg{} }
}

func (r *ResourcePicker) HandleMsg(msg tea.Msg) bool {
	_, ok := msg.(pickerReadyMsg)
	return ok
}

func (r *ResourcePicker) Rows() []table.Row {
	return []table.Row{
		{"EC2 Instances", "Managed instances with SSM agent"},
		{"Auto Scaling Groups", "EC2 Auto Scaling Groups"},
	}
}

func (r *ResourcePicker) Actions(row table.Row) []Action { return nil }

func (r *ResourcePicker) Error() error { return nil }

func (r *ResourcePicker) ChildResource(row table.Row) (string, Resource) {
	switch row[0] {
	case "Auto Scaling Groups":
		return "Auto Scaling Groups", NewASGResource()
	default:
		return "EC2 Instances", NewEC2InstanceResource()
	}
}
