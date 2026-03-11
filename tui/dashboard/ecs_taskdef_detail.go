package dashboard

import (
	"context"
	"fmt"
	"strconv"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsTaskDefDetailFetchMsg struct {
	detail bargeaws.TaskDefDetail
	err    error
}

type ECSTaskDefDetailResource struct {
	family   string
	revision int
	detail   *bargeaws.TaskDefDetail
	err      error
}

func NewECSTaskDefDetailResource(family string, revision int) *ECSTaskDefDetailResource {
	return &ECSTaskDefDetailResource{family: family, revision: revision}
}

func (r *ECSTaskDefDetailResource) Name() string {
	return fmt.Sprintf("%s:%d", r.family, r.revision)
}

func (r *ECSTaskDefDetailResource) Columns() []Column {
	return []Column{
		{"CONTAINER", 25},
		{"IMAGE", 50},
		{"CPU", 6},
		{"MEMORY", 8},
		{"ESSENTIAL", 9},
		{"PORTS", 20},
		{"ENV VARS", 8},
	}
}

func (r *ECSTaskDefDetailResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		detail, err := client.DescribeTaskDefinitionDetail(context.Background(), r.family, r.revision)
		return ecsTaskDefDetailFetchMsg{detail, err}
	}
}

func (r *ECSTaskDefDetailResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsTaskDefDetailFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.detail = &m.detail
		}
		return true
	}
	return false
}

func (r *ECSTaskDefDetailResource) Rows() []table.Row {
	if r.detail == nil {
		return nil
	}
	rows := make([]table.Row, len(r.detail.Containers))
	for i, c := range r.detail.Containers {
		essential := "no"
		if c.Essential {
			essential = "yes"
		}
		rows[i] = table.Row{
			c.Name,
			c.Image,
			strconv.Itoa(int(c.CPU)),
			strconv.Itoa(int(c.Memory)),
			essential,
			c.PortMaps,
			strconv.Itoa(c.EnvCount),
		}
	}
	return rows
}

func (r *ECSTaskDefDetailResource) Actions(row table.Row) []Action { return nil }

func (r *ECSTaskDefDetailResource) Error() error { return r.err }
