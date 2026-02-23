package dashboard

import (
	"context"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsEventsFetchMsg struct {
	events []bargeaws.ServiceEvent
	err    error
}

type ECSEventsResource struct {
	cluster string
	service string
	events  []bargeaws.ServiceEvent
	err     error
}

func NewECSEventsResource(cluster, service string) *ECSEventsResource {
	return &ECSEventsResource{cluster: cluster, service: service}
}

func (r *ECSEventsResource) Name() string { return "Events" }

func (r *ECSEventsResource) Columns() []Column {
	return []Column{
		{"TIME", 20},
		{"MESSAGE", 80},
	}
}

func (r *ECSEventsResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		detail, err := client.DescribeServiceDetail(context.Background(), r.cluster, r.service)
		if err != nil {
			return ecsEventsFetchMsg{nil, err}
		}
		return ecsEventsFetchMsg{detail.Events, nil}
	}
}

func (r *ECSEventsResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsEventsFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.events = m.events
		}
		return true
	}
	return false
}

func (r *ECSEventsResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.events))
	for i, e := range r.events {
		rows[i] = table.Row{
			e.Timestamp.Format("2006-01-02 15:04:05"),
			e.Message,
		}
	}
	return rows
}

func (r *ECSEventsResource) Actions(row table.Row) []Action { return nil }

func (r *ECSEventsResource) Error() error { return r.err }
