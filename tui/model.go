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
	stepMode step = iota
	stepCluster
	stepService
	stepTask
	stepContainer
	stepInstance
	stepDone
)

// Selection holds the user's choices from the TUI drill-down.
type Selection struct {
	Mode       string // "ecs" or "ec2"
	Cluster    string
	Service    string
	Task       string
	Container  string
	InstanceID string
}

// Model is the root Bubble Tea model for the barge TUI.
type Model struct {
	client  *bargeaws.Client
	command string
	ctx     context.Context
	cancel  context.CancelFunc

	step       step
	list       list.Model
	spinner    spinner.Model
	selection  Selection
	modePreset bool // true when mode was set by caller (skip mode selection, back from first step quits)

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
	instances  []bargeaws.InstanceInfo
}

// Messages returned by async commands.
type clustersMsg struct{ clusters []string }
type servicesMsg struct{ services []bargeaws.ServiceInfo }
type tasksMsg struct{ tasks []bargeaws.TaskInfo }
type instancesMsg struct{ instances []bargeaws.InstanceInfo }
type errMsg struct{ err error }

// NewModel creates a new TUI model.
// mode: "" = show mode selection, "ecs" = skip to clusters, "ec2" = skip to instances.
func NewModel(client *bargeaws.Client, command, mode string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	ctx, cancel := context.WithCancel(context.Background())

	m := Model{
		client:  client,
		command: command,
		ctx:     ctx,
		cancel:  cancel,
		spinner: s,
		width:   80,
		height:  24,
	}

	switch mode {
	case "ecs":
		m.modePreset = true
		m.selection.Mode = "ecs"
		m.step = stepCluster
		m.loading = true
	case "ec2":
		m.modePreset = true
		m.selection.Mode = "ec2"
		m.step = stepInstance
		m.loading = true
	default:
		m.step = stepMode
		m.loading = false
		m.list = m.newModeList()
	}

	return m
}

func (m Model) Init() tea.Cmd {
	switch m.step {
	case stepCluster:
		return tea.Batch(m.spinner.Tick, m.fetchClusters())
	case stepInstance:
		return tea.Batch(m.spinner.Tick, m.fetchInstances())
	default:
		return nil
	}
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

	case instancesMsg:
		m.instances = msg.instances
		m.loading = false
		m.autoSelect = ""
		if len(msg.instances) == 0 {
			m.err = fmt.Errorf("no SSM-managed EC2 instances found online")
			return m, nil
		}
		if len(msg.instances) == 1 {
			inst := msg.instances[0]
			label := inst.ID
			if inst.Name != "" {
				label = inst.Name
			}
			m.autoSelect = fmt.Sprintf("Auto-selected instance: %s", label)
			m.selection.InstanceID = inst.ID
			m.step = stepDone
			return m, tea.Quit
		}
		m.list = m.newInstanceList()
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
		case stepInstance:
			label = "instances"
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
		case stepMode:
			m.cancelled = true
			m.cancel()
			return m, tea.Quit

		case stepCluster:
			m.selection.Cluster = ""
			if m.modePreset {
				m.cancelled = true
				m.cancel()
				return m, tea.Quit
			}
			m.step = stepMode
			m.selection.Mode = ""
			m.list = m.newModeList()
			return m, nil

		case stepInstance:
			m.selection.InstanceID = ""
			if m.modePreset {
				m.cancelled = true
				m.cancel()
				return m, tea.Quit
			}
			m.step = stepMode
			m.selection.Mode = ""
			m.list = m.newModeList()
			return m, nil

		case stepService:
			m.step = stepCluster
			m.selection.Service = ""
			if len(m.clusters) == 1 {
				continue // skip auto-selected, keep going back
			}
			m.list = m.newClusterList()
			return m, nil

		case stepTask:
			m.step = stepService
			m.selection.Task = ""
			if len(m.services) == 1 {
				continue
			}
			m.list = m.newServiceList()
			return m, nil

		case stepContainer:
			m.step = stepTask
			m.selection.Container = ""
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
	case stepMode:
		item := selected.(ModeItem)
		m.selection.Mode = item.Mode
		if item.Mode == "ec2" {
			m.step = stepInstance
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchInstances())
		}
		// ECS path
		m.step = stepCluster
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.fetchClusters())

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

	case stepInstance:
		item := selected.(InstanceItem)
		m.selection.InstanceID = item.Info.ID
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

func (m Model) newModeList() list.Model {
	items := []list.Item{
		ModeItem{Mode: "ecs", Label: "ECS Container", Desc: "Connect to an ECS Fargate/EC2 task container"},
		ModeItem{Mode: "ec2", Label: "EC2 Instance", Desc: "Connect to an EC2 instance via SSM Session Manager"},
	}
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height-4)
	l.Title = "Select Target Type"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	return l
}

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

func (m Model) newInstanceList() list.Model {
	items := make([]list.Item, len(m.instances))
	for i, inst := range m.instances {
		items[i] = InstanceItem{Info: inst}
	}
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height-4)
	l.Title = "Select Instance"
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

func (m Model) fetchInstances() tea.Cmd {
	ctx := m.ctx
	client := m.client
	return func() tea.Msg {
		instances, err := client.ListInstances(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return instancesMsg{instances: instances}
	}
}

// --- breadcrumb ---

func (m Model) renderBreadcrumb() string {
	parts := []string{"barge"}

	if m.selection.Mode != "" {
		parts = append(parts, m.selection.Mode)
	}

	// ECS path
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

	// EC2 path
	if m.selection.InstanceID != "" {
		parts = append(parts, m.selection.InstanceID)
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
