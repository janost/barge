package dashboard

import (
	"context"
	"fmt"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsStatusFetchMsg struct {
	rows []statusRow
	err  error
}

type statusRow struct {
	section string
	col1    string
	col2    string
	col3    string
}

type ECSStatusResource struct {
	cluster string
	service string
	rows    []statusRow
	err     error
	client  *bargeaws.Client
}

func NewECSStatusResource(cluster, service string) *ECSStatusResource {
	return &ECSStatusResource{cluster: cluster, service: service}
}

func (r *ECSStatusResource) Name() string { return "Status" }

func (r *ECSStatusResource) Columns() []Column {
	return []Column{
		{"SECTION", 15},
		{"NAME", 30},
		{"STATUS", 15},
		{"DETAIL", 40},
	}
}

func (r *ECSStatusResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	r.client = client
	return func() tea.Msg {
		ctx := context.Background()

		detail, err := client.DescribeServiceDetail(ctx, r.cluster, r.service)
		if err != nil {
			return ecsStatusFetchMsg{nil, err}
		}

		var rows []statusRow

		// Service summary
		rows = append(rows, statusRow{
			section: "Service",
			col1:    detail.Name,
			col2:    detail.Status,
			col3:    fmt.Sprintf("desired:%d running:%d pending:%d", detail.DesiredCount, detail.RunningCount, detail.PendingCount),
		})

		// Deployments
		for _, d := range detail.Deployments {
			rows = append(rows, statusRow{
				section: "Deployment",
				col1:    d.TaskDef,
				col2:    d.Status,
				col3:    fmt.Sprintf("desired:%d running:%d pending:%d updated:%s", d.DesiredCount, d.RunningCount, d.PendingCount, d.UpdatedAt.Format("15:04:05")),
			})
		}

		// Running tasks
		tasks, err := client.ListTasks(ctx, r.cluster, r.service)
		if err == nil {
			for _, t := range tasks {
				started := ""
				if !t.StartedAt.IsZero() {
					started = t.StartedAt.Format("15:04:05")
				}
				rows = append(rows, statusRow{
					section: "Task",
					col1:    t.ID,
					col2:    t.Status,
					col3:    fmt.Sprintf("rev:%s started:%s", t.Revision, started),
				})
			}
		}

		// Stopped tasks (last 24h)
		stopped, err := client.ListStoppedTasks(ctx, r.cluster, r.service)
		if err == nil {
			for _, s := range stopped {
				rows = append(rows, statusRow{
					section: "Stopped",
					col1:    s.ID,
					col2:    s.StoppedAt.Format("15:04:05"),
					col3:    s.StopReason,
				})
			}
		}

		return ecsStatusFetchMsg{rows, nil}
	}
}

func (r *ECSStatusResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsStatusFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.rows = m.rows
		}
		return true
	}
	return false
}

func (r *ECSStatusResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.rows))
	for i, sr := range r.rows {
		rows[i] = table.Row{sr.section, sr.col1, sr.col2, sr.col3}
	}
	return rows
}

func (r *ECSStatusResource) Actions(row table.Row) []Action {
	if r.client == nil {
		return nil
	}
	return []Action{
		NewAPIAction("Force New Deployment", func() error {
			return r.client.ForceNewDeployment(context.Background(), r.cluster, r.service)
		}, "Deployment triggered"),
	}
}

func (r *ECSStatusResource) Error() error { return r.err }
