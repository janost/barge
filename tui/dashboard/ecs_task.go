package dashboard

import (
	"context"
	"fmt"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsTaskFetchMsg struct {
	tasks []bargeaws.TaskInfo
	err   error
}

type ECSTaskResource struct {
	cluster string
	service string
	tasks   []bargeaws.TaskInfo
	err     error
	client  *bargeaws.Client
}

func NewECSTaskResource(cluster, service string) *ECSTaskResource {
	return &ECSTaskResource{cluster: cluster, service: service}
}

func (r *ECSTaskResource) Name() string { return "Tasks" }

func (r *ECSTaskResource) Columns() []Column {
	return []Column{
		{"TASK ID", 20},
		{"STATUS", 10},
		{"TASK DEF", 25},
		{"REVISION", 8},
		{"STARTED", 20},
	}
}

func (r *ECSTaskResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	r.client = client
	return func() tea.Msg {
		tasks, err := client.ListTasks(context.Background(), r.cluster, r.service)
		return ecsTaskFetchMsg{tasks, err}
	}
}

func (r *ECSTaskResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsTaskFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.tasks = m.tasks
		}
		return true
	}
	return false
}

func (r *ECSTaskResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.tasks))
	for i, t := range r.tasks {
		started := ""
		if !t.StartedAt.IsZero() {
			started = t.StartedAt.Format("2006-01-02 15:04:05")
		}
		rows[i] = table.Row{
			t.ID,
			t.Status,
			t.TaskDef,
			t.Revision,
			started,
		}
	}
	return rows
}

func (r *ECSTaskResource) Actions(row table.Row) []Action {
	taskID := row[0]
	for _, t := range r.tasks {
		if t.ID == taskID {
			container, err := bargeaws.ResolveContainer(t.Containers, "")
			if err != nil {
				return nil
			}
			title := fmt.Sprintf("ECS: %s/%s (%s)", r.service, t.ID, container.Name)
			actions := []Action{
				ECSExecAction(r.cluster, t.ID, container.Name, title),
			}
			if r.client != nil {
				taskIDCopy := t.ID
				actions = append(actions, NewAPIAction(
					"Restart Task",
					func() error {
						return r.client.StopTask(context.Background(), r.cluster, taskIDCopy, "Restarted via barge")
					},
					fmt.Sprintf("Task %s restart initiated", taskIDCopy),
				))
			}
			return actions
		}
	}
	return nil
}

func (r *ECSTaskResource) Error() error { return r.err }
