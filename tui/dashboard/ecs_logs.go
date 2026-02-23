package dashboard

import (
	"context"
	"time"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsLogsFetchMsg struct {
	events []bargeaws.LogEvent
	err    error
}

type ECSLogsResource struct {
	cluster string
	service string
	events  []bargeaws.LogEvent
	err     error
}

func NewECSLogsResource(cluster, service string) *ECSLogsResource {
	return &ECSLogsResource{cluster: cluster, service: service}
}

func (r *ECSLogsResource) Name() string { return "Logs" }

func (r *ECSLogsResource) Columns() []Column {
	return []Column{
		{"TIME", 20},
		{"STREAM", 30},
		{"MESSAGE", 80},
	}
}

func (r *ECSLogsResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		logCfg, err := client.ResolveLogConfig(ctx, r.cluster, r.service)
		if err != nil {
			return ecsLogsFetchMsg{nil, err}
		}
		since := time.Now().Add(-15 * time.Minute)
		events, _, err := client.FetchLogs(ctx, logCfg.LogGroup, logCfg.StreamPrefix, "", since, nil)
		return ecsLogsFetchMsg{events, err}
	}
}

func (r *ECSLogsResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsLogsFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.events = m.events
		}
		return true
	}
	return false
}

func (r *ECSLogsResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.events))
	for i, e := range r.events {
		rows[i] = table.Row{
			e.Timestamp.Format("15:04:05.000"),
			e.Stream,
			e.Message,
		}
	}
	return rows
}

func (r *ECSLogsResource) Actions(row table.Row) []Action { return nil }

func (r *ECSLogsResource) Error() error { return r.err }
