package tui

import (
	"context"
	"fmt"
	"strings"

	bargeaws "github.com/MutuallyAssuredDeployment/barge/aws"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type step int

const (
	stepCluster step = iota
	stepService
	stepTask
	stepContainer
	stepDone
)

// Selection holds the user's choices from the TUI drill-down.
type Selection struct {
	Cluster   string
	Service   string
	Task      string
	Container string
}

// Model is the root Bubble Tea model for the barge TUI.
type Model struct {
	client  *bargeaws.Client
	command string
	ctx     context.Context
	cancel  context.CancelFunc

	step      step
	list      list.Model
	spinner   spinner.Model
	selection Selection

	loading    bool
	err        error
	cancelled  bool
	autoSelect string // brief notice when auto-selecting
	width      int
	height     int

	// cached data for back-navigation
	clusters   []string
	services   []bargeaws.ServiceInfo
	tasks      []bargeaws.TaskInfo
	containers []bargeaws.ContainerInfo
}

// Messages returned by async commands.
type clustersMsg struct{ clusters []string }
type servicesMsg struct{ services []bargeaws.ServiceInfo }
type tasksMsg struct{ tasks []bargeaws.TaskInfo }
type errMsg struct{ err error }

func NewModel(client *bargeaws.Client, command string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		client:  client,
		command: command,
		ctx:     ctx,
		cancel:  cancel,
		step:    stepCluster,
		spinner: s,
		loading: true,
		width:   80,
		height:  24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchClusters())
}

func (m Model) Cancelled() bool     { return m.cancelled }
func (m Model) Selection() Selection { return m.selection }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.loading {
			m.list.SetSize(msg.Width, msg.Height-4)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.list.FilterState() == list.Filtering {
				break
			}
			m.cancelled = true
			m.cancel()
			return m, tea.Quit
		case "esc":
			if m.list.FilterState() == list.Filtering {
				break
			}
			return m.goBack()
		case "enter":
			if m.list.FilterState() == list.Filtering {
				break
			}
			if !m.loading && m.err == nil {
				return m.selectCurrent()
			}
			return m, nil
		}

	case clustersMsg:
		m.clusters = msg.clusters
		m.loading = false
		m.autoSelect = ""
		if len(msg.clusters) == 0 {
			m.err = fmt.Errorf("no ECS clusters found in this account/region")
			return m, nil
		}
		if len(msg.clusters) == 1 {
			m.autoSelect = fmt.Sprintf("Auto-selected cluster: %s", msg.clusters[0])
			m.selection.Cluster = msg.clusters[0]
			m.step = stepService
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchServices())
		}
		m.list = m.newClusterList()
		return m, nil

	case servicesMsg:
		m.services = msg.services
		m.loading = false
		m.autoSelect = ""
		if len(msg.services) == 0 {
			m.err = fmt.Errorf("no services found in cluster %q", m.selection.Cluster)
			return m, nil
		}
		if len(msg.services) == 1 {
			m.autoSelect = fmt.Sprintf("Auto-selected service: %s", msg.services[0].Name)
			m.selection.Service = msg.services[0].Name
			m.step = stepTask
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchTasks())
		}
		m.list = m.newServiceList()
		return m, nil

	case tasksMsg:
		m.tasks = msg.tasks
		m.loading = false
		m.autoSelect = ""
		if len(msg.tasks) == 0 {
			m.err = fmt.Errorf("no running tasks for service %q", m.selection.Service)
			return m, nil
		}
		if len(msg.tasks) == 1 {
			t := msg.tasks[0]
			m.autoSelect = fmt.Sprintf("Auto-selected task: %s", t.ID)
			m.selection.Task = t.ID
			m.containers = t.Containers
			return m.advanceToContainers(t.Containers)
		}
		m.list = m.newTaskList()
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Delegate to the list
	if !m.loading && m.err == nil {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(m.renderBreadcrumb())
	b.WriteString("\n")

	if m.autoSelect != "" {
		b.WriteString(autoSelectStyle.Render(m.autoSelect))
		b.WriteString("\n")
	}

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.err)))
		b.WriteString("\n")
		b.WriteString(statusStyle.Render("esc back • q quit"))
		return b.String()
	}

	if m.loading {
		label := "clusters"
		switch m.step {
		case stepService:
			label = "services"
		case stepTask:
			label = "tasks"
		case stepContainer:
			label = "containers"
		}
		b.WriteString(fmt.Sprintf("  %s Loading %s...\n", m.spinner.View(), label))
		return b.String()
	}

	b.WriteString(m.list.View())
	return b.String()
}

// --- navigation ---

// goBack walks backward through the drill-down, skipping auto-selected steps.
func (m Model) goBack() (Model, tea.Cmd) {
	m.err = nil
	m.autoSelect = ""

	for {
		switch m.step {
		case stepCluster:
			m.cancelled = true
			m.cancel()
			return m, tea.Quit

		case stepService:
			m.step = stepCluster
			m.selection.Cluster = ""
			if len(m.clusters) == 1 {
				continue // skip auto-selected, keep going back
			}
			m.list = m.newClusterList()
			return m, nil

		case stepTask:
			m.step = stepService
			m.selection.Service = ""
			if len(m.services) == 1 {
				continue
			}
			m.list = m.newServiceList()
			return m, nil

		case stepContainer:
			m.step = stepTask
			m.selection.Task = ""
			if len(m.tasks) == 1 {
				continue
			}
			m.list = m.newTaskList()
			return m, nil

		default:
			return m, nil
		}
	}
}

func (m Model) selectCurrent() (Model, tea.Cmd) {
	selected := m.list.SelectedItem()
	if selected == nil {
		return m, nil
	}

	switch m.step {
	case stepCluster:
		item := selected.(ClusterItem)
		m.selection.Cluster = item.Name
		m.step = stepService
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.fetchServices())

	case stepService:
		item := selected.(ServiceItem)
		m.selection.Service = item.Info.Name
		m.step = stepTask
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.fetchTasks())

	case stepTask:
		item := selected.(TaskItem)
		m.selection.Task = item.Info.ID
		m.containers = item.Info.Containers
		return m.advanceToContainers(item.Info.Containers)

	case stepContainer:
		item := selected.(ContainerItem)
		m.selection.Container = item.Info.Name
		m.step = stepDone
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) advanceToContainers(containers []bargeaws.ContainerInfo) (Model, tea.Cmd) {
	if len(containers) == 1 {
		m.autoSelect = fmt.Sprintf("Auto-selected container: %s", containers[0].Name)
		m.selection.Container = containers[0].Name
		m.step = stepDone
		return m, tea.Quit
	}

	m.step = stepContainer
	m.list = m.newContainerList(containers)

	// Pre-highlight the essential container if there's exactly one
	essentialIdx := -1
	essentialCount := 0
	for i, c := range containers {
		if c.Essential {
			essentialIdx = i
			essentialCount++
		}
	}
	if essentialCount == 1 {
		m.list.Select(essentialIdx)
	}

	return m, nil
}

// --- list constructors ---

func (m Model) newClusterList() list.Model {
	items := make([]list.Item, len(m.clusters))
	for i, c := range m.clusters {
		items[i] = ClusterItem{Name: c}
	}
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height-4)
	l.Title = "Select Cluster"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	return l
}

func (m Model) newServiceList() list.Model {
	items := make([]list.Item, len(m.services))
	for i, s := range m.services {
		items[i] = ServiceItem{Info: s}
	}
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height-4)
	l.Title = "Select Service"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	return l
}

func (m Model) newTaskList() list.Model {
	items := make([]list.Item, len(m.tasks))
	for i, t := range m.tasks {
		items[i] = TaskItem{Info: t}
	}
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height-4)
	l.Title = "Select Task"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	return l
}

func (m Model) newContainerList(containers []bargeaws.ContainerInfo) list.Model {
	items := make([]list.Item, len(containers))
	for i, c := range containers {
		items[i] = ContainerItem{Info: c}
	}
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height-4)
	l.Title = "Select Container"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	return l
}

// --- async commands ---

func (m Model) fetchClusters() tea.Cmd {
	ctx := m.ctx
	client := m.client
	return func() tea.Msg {
		clusters, err := client.ListClusters(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return clustersMsg{clusters: clusters}
	}
}

func (m Model) fetchServices() tea.Cmd {
	ctx := m.ctx
	client := m.client
	cluster := m.selection.Cluster
	return func() tea.Msg {
		services, err := client.ListServices(ctx, cluster)
		if err != nil {
			return errMsg{err: err}
		}
		return servicesMsg{services: services}
	}
}

func (m Model) fetchTasks() tea.Cmd {
	ctx := m.ctx
	client := m.client
	cluster := m.selection.Cluster
	service := m.selection.Service
	return func() tea.Msg {
		tasks, err := client.ListTasks(ctx, cluster, service)
		if err != nil {
			return errMsg{err: err}
		}
		return tasksMsg{tasks: tasks}
	}
}

// --- breadcrumb ---

func (m Model) renderBreadcrumb() string {
	parts := []string{"barge"}

	if m.selection.Cluster != "" {
		parts = append(parts, m.selection.Cluster)
	}
	if m.selection.Service != "" {
		parts = append(parts, m.selection.Service)
	}
	if m.selection.Task != "" {
		parts = append(parts, m.selection.Task)
	}
	if m.selection.Container != "" {
		parts = append(parts, m.selection.Container)
	}

	if len(parts) == 1 {
		return breadcrumbStyle.Render("barge")
	}

	rendered := make([]string, len(parts))
	for i, p := range parts {
		if i == len(parts)-1 {
			rendered[i] = breadcrumbActiveStyle.Render(p)
		} else {
			rendered[i] = breadcrumbStyle.Render(p)
		}
	}
	return strings.Join(rendered, breadcrumbStyle.Render(" > "))
}
