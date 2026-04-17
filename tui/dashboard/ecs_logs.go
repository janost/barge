package dashboard

import (
	"context"
	"time"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsLogsFetchMsg struct {
	events []bridgeaws.LogEvent
	err    error
}

type ECSLogsResource struct {
	cluster   string
	service   string
	events    []bridgeaws.LogEvent
	err       error
	lastFetch time.Time
}

func NewECSLogsResource(cluster, service string) *ECSLogsResource {
	return &ECSLogsResource{cluster: cluster, service: service}
}

func (r *ECSLogsResource) Name() string { return "Logs (tailing)" }

func (r *ECSLogsResource) Columns() []Column {
	return []Column{
		{"TIME", 20},
		{"STREAM", 30},
		{"MESSAGE", 80},
	}
}

func (r *ECSLogsResource) FetchCmd(client *bridgeaws.Client) tea.Cmd {
	since := r.lastFetch
	if since.IsZero() {
		since = time.Now().Add(-15 * time.Minute)
	} else {
		// Nudge forward 1ms to avoid re-fetching the last event (startTime is inclusive)
		since = since.Add(time.Millisecond)
	}
	return func() tea.Msg {
		ctx := context.Background()
		logCfg, err := client.ResolveLogConfig(ctx, r.cluster, r.service)
		if err != nil {
			return ecsLogsFetchMsg{nil, err}
		}
		events, _, err := client.FetchLogs(ctx, logCfg.LogGroup, logCfg.StreamPrefix, "", since, nil)
		return ecsLogsFetchMsg{events, err}
	}
}

func (r *ECSLogsResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsLogsFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			if r.lastFetch.IsZero() {
				// Initial fetch — replace
				r.events = m.events
			} else {
				// Tail — append new events
				r.events = append(r.events, m.events...)
			}
			// Cap to prevent unbounded memory growth
			const maxEvents = 10000
			if len(r.events) > maxEvents {
				r.events = r.events[len(r.events)-maxEvents:]
			}
			r.lastFetch = time.Now()
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

func (r *ECSLogsResource) TailInterval() time.Duration {
	return 5 * time.Second
}

func (r *ECSLogsResource) Error() error { return r.err }
