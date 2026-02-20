// tui/dashboard/model.go
package dashboard

import (
	"fmt"
	"strings"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type state int

const (
	stateLoading state = iota
	stateTable
	stateActions
)

// Model is the main dashboard Bubble Tea model.
type Model struct {
	client        *bargeaws.Client
	resourceStack []Resource
	breadcrumbs   []string
	table         table.Model
	baseColumns   []Column
	state         state

	// Action menu
	actions   []Action
	actionIdx int

	// Layout
	width  int
	height int

	// Status
	message string
}

func (m *Model) currentResource() Resource {
	return m.resourceStack[len(m.resourceStack)-1]
}

// New creates a new dashboard model with the given resource.
func New(client *bargeaws.Client, resource Resource) Model {
	columns := resource.Columns()
	tableCols := make([]table.Column, len(columns))
	for i, c := range columns {
		tableCols[i] = table.Column{Title: c.Title, Width: c.Width}
	}

	t := table.New(
		table.WithColumns(tableCols),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(20),
	)
	t.SetStyles(tableStyles())

	return Model{
		client:        client,
		resourceStack: []Resource{resource},
		breadcrumbs:   []string{resource.Name()},
		baseColumns:   columns,
		table:         t,
		state:         stateLoading,
	}
}

// resizeColumns distributes the terminal width across table columns proportionally.
func (m *Model) resizeColumns() {
	if m.width <= 0 || len(m.baseColumns) == 0 {
		return
	}
	totalBase := 0
	for _, c := range m.baseColumns {
		totalBase += c.Width
	}
	available := m.width
	cols := make([]table.Column, len(m.baseColumns))
	remaining := available
	for i, c := range m.baseColumns {
		if i == len(m.baseColumns)-1 {
			cols[i] = table.Column{Title: c.Title, Width: remaining}
		} else {
			w := available * c.Width / totalBase
			cols[i] = table.Column{Title: c.Title, Width: w}
			remaining -= w
		}
	}
	m.table.SetColumns(cols)
}

func (m *Model) reconfigureTable(res Resource) {
	columns := res.Columns()
	m.baseColumns = columns
	tableCols := make([]table.Column, len(columns))
	for i, c := range columns {
		tableCols[i] = table.Column{Title: c.Title, Width: c.Width}
	}
	m.table.SetColumns(tableCols)
	m.resizeColumns()
}

func (m Model) Init() tea.Cmd {
	return m.currentResource().FetchCmd(m.client)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// 3 lines for header+shortcuts+blank, 1 for status bar
		m.table.SetHeight(msg.Height - 4)
		m.resizeColumns()
		return m, nil

	case processExitMsg:
		m.state = stateTable
		if msg.err != nil {
			m.message = fmt.Sprintf("Process error: %v", msg.err)
		} else {
			m.message = "Session ended."
		}
		return m, m.currentResource().FetchCmd(m.client)

	case tea.KeyMsg:
		switch m.state {
		case stateLoading:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		case stateTable:
			return m.updateTable(msg)
		case stateActions:
			return m.updateActions(msg)
		}
	}

	// Let resource handle data messages
	if m.currentResource().HandleMsg(msg) {
		m.table.SetRows(m.currentResource().Rows())
		m.state = stateTable
		m.message = ""
		return m, nil
	}

	// Pass through to table for cursor movement etc.
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "enter":
		row := m.table.SelectedRow()
		if row == nil {
			return m, nil
		}
		if drillable, ok := m.currentResource().(Drillable); ok {
			label, child := drillable.ChildResource(row)
			m.resourceStack = append(m.resourceStack, child)
			m.breadcrumbs = append(m.breadcrumbs, label)
			m.reconfigureTable(child)
			m.state = stateLoading
			return m, child.FetchCmd(m.client)
		}
		m.actions = m.currentResource().Actions(row)
		m.actionIdx = 0
		m.state = stateActions
		return m, nil
	case "esc", "backspace":
		if len(m.resourceStack) > 1 {
			m.resourceStack = m.resourceStack[:len(m.resourceStack)-1]
			m.breadcrumbs = m.breadcrumbs[:len(m.breadcrumbs)-1]
			parent := m.currentResource()
			m.reconfigureTable(parent)
			m.table.SetRows(parent.Rows())
			m.state = stateTable
			m.message = ""
			return m, nil
		}
		return m, nil
	case "r":
		m.state = stateLoading
		m.message = ""
		return m, m.currentResource().FetchCmd(m.client)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) updateActions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.state = stateTable
		return m, nil
	case "enter":
		if m.actionIdx < len(m.actions) {
			return m, m.actions[m.actionIdx].Run()
		}
		return m, nil
	case "up", "k":
		if m.actionIdx > 0 {
			m.actionIdx--
		}
		return m, nil
	case "down", "j":
		if m.actionIdx < len(m.actions)-1 {
			m.actionIdx++
		}
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	// Header line
	title := headerStyle.Render(" barge")
	title += " ▸ " + resourceStyle.Render(m.currentResource().Name())
	if m.state == stateActions {
		row := m.table.SelectedRow()
		if row != nil {
			title += " ▸ " + selectedStyle.Render(row[0])
			if len(row) > 1 && row[1] != "" {
				title += " " + countStyle.Render("("+row[1]+")")
			}
		}
	} else {
		count := len(m.currentResource().Rows())
		title += "  " + countStyle.Render(fmt.Sprintf("%d items", count))
	}
	b.WriteString(title + "\n")

	// Shortcuts line
	if m.state == stateActions {
		b.WriteString(shortcutStyle.Render(" [Enter] Select  [Esc] Back") + "\n")
	} else {
		b.WriteString(shortcutStyle.Render(" [↑↓] Navigate  [Enter] Actions  [r] Refresh  [q] Quit") + "\n")
	}
	b.WriteString("\n")

	// Main content
	switch m.state {
	case stateLoading:
		b.WriteString(statusStyle.Render(" Loading...") + "\n")
	case stateActions:
		b.WriteString(m.renderActions())
	default:
		if err := m.currentResource().Error(); err != nil {
			b.WriteString(errorStyle.Render(" Error: "+err.Error()) + "\n")
		} else {
			b.WriteString(m.table.View() + "\n")
		}
	}

	// Status bar
	if m.message != "" {
		b.WriteString("\n" + statusStyle.Render(" "+m.message))
	}

	return b.String()
}

func (m Model) renderActions() string {
	var b strings.Builder
	for i, action := range m.actions {
		if i == m.actionIdx {
			b.WriteString(actionCursorStyle.Render("  ▸ "+action.Name) + "\n")
		} else {
			b.WriteString(actionNormalStyle.Render("  "+action.Name) + "\n")
		}
	}
	return b.String()
}
