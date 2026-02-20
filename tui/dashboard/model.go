// tui/dashboard/model.go
package dashboard

import (
	"fmt"
	"strings"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	available := m.width - 2 // subtract border chars
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
		// 2 for top/bottom border, 1 for table header, 1 for status bar
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

func (m Model) renderBreadcrumb() string {
	parts := make([]string, len(m.breadcrumbs))
	for i, b := range m.breadcrumbs {
		if i == len(m.breadcrumbs)-1 {
			parts[i] = resourceStyle.Render(b)
		} else {
			parts[i] = countStyle.Render(b)
		}
	}
	return strings.Join(parts, sepStyle.Render(" ▸ "))
}

func (m Model) renderBorderedPanel(title, content, shortcuts string) string {
	border := lipgloss.RoundedBorder()
	bStyle := lipgloss.NewStyle().Foreground(borderFg)
	w := m.width
	if w < 4 {
		return content
	}

	// Top line: ╭─ Title ──────────────────╮
	titleRendered := " " + title + " "
	titleWidth := lipgloss.Width(titleRendered)
	topFill := w - titleWidth - 3 // 2 corners + 1 leading border char
	if topFill < 0 {
		topFill = 0
	}
	topLine := bStyle.Render(border.TopLeft) +
		bStyle.Render(border.Top) +
		titleRendered +
		bStyle.Render(strings.Repeat(border.Top, topFill)) +
		bStyle.Render(border.TopRight)

	// Content lines: │ content │
	contentLines := strings.Split(content, "\n")
	innerWidth := w - 2 // 2 for side border chars
	var body strings.Builder
	for _, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		pad := innerWidth - lineWidth
		if pad < 0 {
			pad = 0
		}
		body.WriteString(
			bStyle.Render(border.Left) +
				line + strings.Repeat(" ", pad) +
				bStyle.Render(border.Right) + "\n")
	}

	// Bottom line: ╰─ shortcuts ──────────────╯
	shortRendered := " " + shortcuts + " "
	shortWidth := lipgloss.Width(shortRendered)
	botFill := w - shortWidth - 3 // 2 corners + 1 leading border char
	if botFill < 0 {
		botFill = 0
	}
	botLine := bStyle.Render(border.BottomLeft) +
		bStyle.Render(border.Bottom) +
		shortRendered +
		bStyle.Render(strings.Repeat(border.Bottom, botFill)) +
		bStyle.Render(border.BottomRight)

	return topLine + "\n" + body.String() + botLine
}

func (m Model) View() string {
	// Build breadcrumb title
	title := m.renderBreadcrumb()
	if m.state == stateActions {
		row := m.table.SelectedRow()
		if row != nil {
			title += sepStyle.Render(" ▸ ") + selectedStyle.Render(row[0])
		}
	} else {
		count := len(m.currentResource().Rows())
		title += "  " + countStyle.Render(fmt.Sprintf("[%d]", count))
	}

	// Build shortcuts
	var shortcuts string
	if m.state == stateActions {
		shortcuts = shortcutStyle.Render("Enter:Select  Esc:Back")
	} else {
		nav := "↑↓:Navigate  Enter:Select  r:Refresh  q:Quit"
		if len(m.resourceStack) > 1 {
			nav = "↑↓:Navigate  Enter:Select  Esc:Back  r:Refresh  q:Quit"
		}
		shortcuts = shortcutStyle.Render(nav)
	}

	// Build inner content
	var content string
	switch m.state {
	case stateLoading:
		content = statusStyle.Render("Loading...")
	case stateActions:
		bg := m.table.View()
		popup := m.renderActionPopup()
		bgH := strings.Count(bg, "\n") + 1
		content = overlayCenter(bg, popup, m.width-2, bgH)
	default:
		if err := m.currentResource().Error(); err != nil {
			content = errorStyle.Render("Error: " + err.Error())
		} else {
			content = m.table.View()
		}
	}

	// Render bordered panel
	panel := m.renderBorderedPanel(title, content, shortcuts)

	// Status bar below
	if m.message != "" {
		panel += "\n" + statusStyle.Render(" "+m.message)
	}

	return panel
}

func overlayCenter(bg, fg string, bgW, bgH int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	fgW := lipgloss.Width(fg)
	fgH := len(fgLines)

	x := max((bgW-fgW)/2, 0)
	y := max((bgH-fgH)/2, 0)

	for i, fgLine := range fgLines {
		row := y + i
		if row >= len(bgLines) {
			break
		}
		left := ansi.Truncate(bgLines[row], x, "")
		leftW := lipgloss.Width(left)
		if leftW < x {
			left += strings.Repeat(" ", x-leftW)
		}
		right := ansi.TruncateLeft(bgLines[row], x+lipgloss.Width(fgLine), "")
		bgLines[row] = left + "\x1b[m" + fgLine + "\x1b[m" + right
	}

	return strings.Join(bgLines, "\n")
}

func (m Model) renderActionPopup() string {
	var b strings.Builder
	for i, action := range m.actions {
		if i == m.actionIdx {
			b.WriteString(actionCursorStyle.Render("▸ " + action.Name))
		} else {
			b.WriteString(actionNormalStyle.Render("  " + action.Name))
		}
		if i < len(m.actions)-1 {
			b.WriteString("\n")
		}
	}
	return popupStyle.Render(b.String())
}
