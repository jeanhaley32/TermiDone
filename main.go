package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	configDir  = "~/.config/termidone"
	tasksFile  = "tasks.json"
	reportFile = "work_report_%s.md"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF")).
		Background(lipgloss.Color("#5D5B60")).
		Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
		Render

	detailBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		Padding(1, 2)

	helpStyle = lipgloss.NewStyle().Padding(1, 0)

	helpKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	helpDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	columnStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Width(25)

	focusedColumnStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Width(25)

	taskStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	completedTaskStyle = lipgloss.NewStyle().
		Strikethrough(true).
		Foreground(lipgloss.Color("240"))
)

// Task represents a single TODO item
type Task struct {
	ID           int           `json:"id"`
	TaskTitle    string        `json:"title"`
	Completed    bool          `json:"completed"`
	CreatedAt    time.Time     `json:"created_at"`
	CompletedAt  time.Time     `json:"completed_at"`
	WorkSessions []WorkSession `json:"work_sessions"`
	Column       KanbanColumn  `json:"column"`
}

// WorkSession represents a single work session for a task
type WorkSession struct {
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
}

// AppState represents the different states of the application
type AppState int

const (
	StateViewTasks AppState = iota
	StateAddTask
	StateTimer
	StateHelp
	StateViewTask
)

// MainView represents the two main views of the application
type MainView int

const (
	ListView MainView = iota
	KanbanView
)

// Implement the list.Item interface for Task
func (t Task) FilterValue() string { return t.TaskTitle }
func (t Task) Title() string {
	if t.Completed {
		return fmt.Sprintf("✓ %s", t.TaskTitle)
	}
	return t.TaskTitle
}
func (t Task) Description() string {
	if t.Completed {
		return fmt.Sprintf("Completed on %s", t.CompletedAt.Format("Jan 02, 2006"))
	}
	return fmt.Sprintf("Created on %s", t.CreatedAt.Format("Jan 02, 2006"))
}

// KanbanColumn represents the column a task is in
type KanbanColumn int

const (
	ColumnTodo KanbanColumn = iota
	ColumnInProgress
	ColumnDone
)

// Model represents the application's state
type model struct {
	tasks         []Task
	list          list.Model
	textInput     textinput.Model
	state         AppState
	mainView      MainView
	quitting      bool
	currentTask   *Task // The task currently being worked on
	timer         *time.Timer
	timerDuration time.Duration
	timerStartTime time.Time
	statusMessage string
	selectedColumn KanbanColumn
	selectedTaskIndex [3]int // 0: Todo, 1: InProgress, 2: Done
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Add a new task..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	m := model{
		textInput:     ti,
		state:         StateViewTasks,
		mainView:      ListView,
		timerDuration: 25 * time.Minute, // Default Pomodoro duration
		selectedColumn: ColumnTodo,
		selectedTaskIndex: [3]int{0, 0, 0},
	}

	m.loadTasks()
	m.updateListItems()

	return m
}

// loadTasks loads tasks from the JSON file
func (m *model) loadTasks() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		m.statusMessage = fmt.Sprintf("Error getting home directory: %v", err)
		return
	}
	configPath := fmt.Sprintf("%s/.config/termidone", homeDir)
	filePath := fmt.Sprintf("%s/tasks.json", configPath)

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			m.tasks = []Task{}
			return
		}
		m.statusMessage = fmt.Sprintf("Error reading tasks file: %v", err)
		return
	}

	err = json.Unmarshal(data, &m.tasks)
	if err != nil {
		m.statusMessage = fmt.Sprintf("Error unmarshaling tasks: %v", err)
		m.tasks = []Task{} // Reset tasks if unmarshaling fails
	}

	// Ensure all tasks have a column, default to ColumnTodo if not set
	for i := range m.tasks {
		if m.tasks[i].Column == 0 && !m.tasks[i].Completed {
			m.tasks[i].Column = ColumnTodo
		} else if m.tasks[i].Completed {
			m.tasks[i].Column = ColumnDone
		}
	}
}

// saveTasks saves tasks to the JSON file
func (m *model) saveTasks() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		m.statusMessage = fmt.Sprintf("Error getting home directory: %v", err)
		return
	}
	configPath := fmt.Sprintf("%s/.config/termidone", homeDir)
	filePath := fmt.Sprintf("%s/tasks.json", configPath)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		os.MkdirAll(configPath, 0755)
	}

	data, err := json.MarshalIndent(m.tasks, "", "  ")
	if err != nil {
		m.statusMessage = fmt.Sprintf("Error marshaling tasks: %v", err)
		return
	}

	err = ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		m.statusMessage = fmt.Sprintf("Error writing tasks file: %v", err)
	}
}

// updateListItems updates the list.Model with current tasks
func (m *model) updateListItems() {
	items := make([]list.Item, len(m.tasks))
	for i, task := range m.tasks {
		items[i] = task
	}
	m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
	m.list.Title = "TermiDone Tasks"
	// Set a more prominent highlight for the selected item
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F7931E", Dark: "#F7931E"}).
		Foreground(lipgloss.AdaptiveColor{Light: "#EE6123", Dark: "#EE6123"}).
		Padding(0, 0, 0, 2)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy()
	m.list.SetDelegate(delegate)
}

func (m model) Init() tea.Cmd {
	return m.textInput.Focus()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 5) // Adjust height for status/input
		return m, nil
	case tea.KeyMsg:
		// Global keybindings
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if msg.String() == "tab" {
			if m.state == StateViewTasks {
				if m.mainView == ListView {
					m.mainView = KanbanView
				} else {
					m.mainView = ListView
				}
			}
			return m, nil
		}

		switch m.state {
		case StateViewTasks:
			switch m.mainView {
			case ListView:
				return m.updateListView(msg)
			case KanbanView:
				return m.updateKanbanView(msg)
			}
		case StateAddTask:
			switch msg.String() {
			case "enter":
				newTaskTitle := m.textInput.Value()
				if newTaskTitle != "" {
					newID := 1
					if len(m.tasks) > 0 {
						newID = m.tasks[len(m.tasks)-1].ID + 1
					}
					m.tasks = append(m.tasks, Task{
						ID:        newID,
						TaskTitle:     newTaskTitle,
						Completed: false,
						CreatedAt: time.Now(),
						Column:    ColumnTodo,
					})
					m.saveTasks()
					m.updateListItems()
					m.statusMessage = fmt.Sprintf("Added task: %s", newTaskTitle)
				}
				m.state = StateViewTasks
				return m, nil
			case "esc":
				m.state = StateViewTasks
				return m, nil
			}
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)
		case StateTimer:
			switch msg.String() {
			case "q", "esc": // Allow quitting/stopping timer
				if m.timer != nil {
					m.timer.Stop()
				}
				m.state = StateViewTasks
				m.currentTask = nil
				return m, nil
			}
		case StateHelp:
			switch msg.String() {
			case "?", "esc", "q":
				m.state = StateViewTasks
				return m, nil
			}
		case StateViewTask:
			switch msg.String() {
			case "esc", "q":
				m.state = StateViewTasks
				m.currentTask = nil
				return m, nil
			}
		}
	case TimerTickMsg:
		if m.state == StateTimer && m.currentTask != nil {
			elapsed := time.Since(m.timerStartTime)
			if elapsed >= m.timerDuration {
				// Timer finished, mark task as complete
				if m.currentTask != nil {
					for i := range m.tasks {
						if m.tasks[i].ID == m.currentTask.ID {
															m.tasks[i].Completed = true
								m.tasks[i].CompletedAt = time.Now()
								m.tasks[i].Column = ColumnDone // Set column to Done when completed
								// Add work session
								m.tasks[i].WorkSessions = append(m.tasks[i].WorkSessions, WorkSession{
									Start: m.timerStartTime,
									End:   time.Now(),
									Duration: m.timerDuration,
								})
								break
						}
					}
					m.saveTasks()
					m.updateListItems()
					m.statusMessage = fmt.Sprintf("Timer finished for task: %s", m.currentTask.TaskTitle)
				}
				m.state = StateViewTasks
				m.currentTask = nil
				if m.timer != nil {
					m.timer.Stop()
				}
				return m, nil
			}
			// Continue ticking
			cmds = append(cmds, tea.Tick(time.Second, func(t time.Time) tea.Msg {
				return TimerTickMsg(t)
			}))
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "n": // New task
			m.state = StateAddTask
			m.textInput.Reset()
			return m, m.textInput.Focus()
		case "d": // Delete task
			if len(m.tasks) > 0 {
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					selectedTask, ok := selectedItem.(Task)
					if ok {
						for i, task := range m.tasks {
							if task.ID == selectedTask.ID {
								m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
								break
							}
						}
						m.saveTasks()
						m.updateListItems()
						m.statusMessage = fmt.Sprintf("Deleted task: %s", selectedTask.TaskTitle)
					}
				}
			}
		case "c": // Complete task
			if len(m.tasks) > 0 {
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					selectedTask, ok := selectedItem.(Task)
					if ok && !selectedTask.Completed {
						for i := range m.tasks {
							if m.tasks[i].ID == selectedTask.ID {
								m.tasks[i].Completed = true
								m.tasks[i].CompletedAt = time.Now()
								m.tasks[i].Column = ColumnDone // Set column to Done when completed
																	m.tasks[i].Completed = true
									m.tasks[i].CompletedAt = time.Now()
									m.tasks[i].Column = ColumnDone // Set column to Done when completed
									break
								}
							}
							m.saveTasks()
							m.updateListItems()
							m.statusMessage = fmt.Sprintf("Completed task: %s", selectedTask.TaskTitle)
						}
				}
			}
		case "t": // Start timer for selected task
			if len(m.tasks) > 0 {
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					selectedTask, ok := selectedItem.(Task)
					if ok && !selectedTask.Completed {
						taskCopy := selectedTask
						m.currentTask = &taskCopy
						m.state = StateTimer
						m.timerStartTime = time.Now()
						m.timer = time.NewTimer(m.timerDuration)
						return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
							return TimerTickMsg(t)
						})
					}
				}
			}
		case "r": // Generate report
			m.generateReport()
		case "?":
			m.state = StateHelp
		case "enter":
			if len(m.tasks) > 0 {
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					selectedTask, ok := selectedItem.(Task)
					if ok {
						taskCopy := selectedTask
						m.currentTask = &taskCopy
						m.state = StateViewTask
					}
				}
			}
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) updateKanbanView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Get tasks for the currently selected column
	currentColumnTasks := []Task{}
	for _, task := range m.tasks {
		if task.Column == m.selectedColumn {
			currentColumnTasks = append(currentColumnTasks, task)
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "left", "h":
			if m.selectedColumn > ColumnTodo {
				m.selectedColumn--
				m.statusMessage = fmt.Sprintf("Switched to %s column", m.columnName(m.selectedColumn))
			}
		case "right", "l":
			if m.selectedColumn < ColumnDone {
				m.selectedColumn++
				m.statusMessage = fmt.Sprintf("Switched to %s column", m.columnName(m.selectedColumn))
			}
		case "up", "k":
			if len(currentColumnTasks) > 0 && m.selectedTaskIndex[m.selectedColumn] > 0 {
				m.selectedTaskIndex[m.selectedColumn]--
			}
		case "down", "j":
			if len(currentColumnTasks) > 0 && m.selectedTaskIndex[m.selectedColumn] < len(currentColumnTasks)-1 {
				m.selectedTaskIndex[m.selectedColumn]++
			}
		case "enter":
			if len(currentColumnTasks) > 0 {
				selectedTask := currentColumnTasks[m.selectedTaskIndex[m.selectedColumn]]
				for i := range m.tasks {
					if m.tasks[i].ID == selectedTask.ID {
						if m.tasks[i].Column < ColumnDone {
							m.tasks[i].Column++
							if m.tasks[i].Column == ColumnDone {
								m.tasks[i].Completed = true
								m.tasks[i].CompletedAt = time.Now()
							}
							m.saveTasks()
							m.statusMessage = fmt.Sprintf("Moved task \"%s\" to %s", m.tasks[i].TaskTitle, m.columnName(m.tasks[i].Column))
							// Reset selected task index for the old column if it's out of bounds
							// This is a simplified approach; a more robust solution might re-evaluate all indices.
							if m.selectedTaskIndex[m.selectedColumn] >= len(currentColumnTasks)-1 && len(currentColumnTasks) > 1 {
								m.selectedTaskIndex[m.selectedColumn]--
							}
							break
						}
					}
				}
			} else {
				m.statusMessage = "No tasks to move in this column."
			}
		}
	}
	return m, nil
}

func (m model) columnName(col KanbanColumn) string {
	switch col {
	case ColumnTodo:
		return "TODO"
	case ColumnInProgress:
		return "IN PROGRESS"
	case ColumnDone:
		return "DONE"
	}
	return "Unknown"
}

func (m model) View() string {
	if m.quitting {
		return "Exiting TermiDone. Goodbye!\n"
	}

	switch m.state {
	case StateViewTasks:
		switch m.mainView {
		case ListView:
			return m.viewListView()
		case KanbanView:
			return m.viewKanbanView()
		}
	case StateAddTask:
		return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Add New Task"),
			"What is your new task?\n",
			m.textInput.View(),
			"\n(Press 'enter' to save, 'esc' to cancel)",
		))
	case StateTimer:
		if m.currentTask == nil {
			m.state = StateViewTasks // Fallback if no task is set
			return m.View()
		}
		elapsed := time.Since(m.timerStartTime)
		remaining := m.timerDuration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Work Timer"),
			fmt.Sprintf("\nWorking on: %s", m.currentTask.TaskTitle),
			fmt.Sprintf("\nTime remaining: %s", formatDuration(remaining)),
			"\n(Press 'esc' or 'q' to stop timer)",
		))
	case StateHelp:
		return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Help"),
			helpStyle.Render(m.helpView()),
			"\n(Press '?', 'q', or 'esc' to return)",
		))
	case StateViewTask:
		if m.currentTask == nil {
			m.state = StateViewTasks // Fallback if no task is set
			return m.View()
		}
		return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Task Details"),
			detailBoxStyle.Render(
				fmt.Sprintf("Title: %s\n", m.currentTask.TaskTitle) +
					fmt.Sprintf("Created: %s\n", m.currentTask.CreatedAt.Format("Jan 02, 2006 15:04:05")) +
					fmt.Sprintf("Completed: %v\n", m.currentTask.Completed) +
					func() string {
						if m.currentTask.Completed {
							return fmt.Sprintf("Completed At: %s\n", m.currentTask.CompletedAt.Format("Jan 02, 2006 15:04:05"))
						}
						return ""
					}() +
					func() string {
						if len(m.currentTask.WorkSessions) > 0 {
							ws := "\nWork Sessions:\n"
							for _, s := range m.currentTask.WorkSessions {
								ws += fmt.Sprintf("  - %s to %s (%.f minutes)\n", s.Start.Format("15:04"), s.End.Format("15:04"), s.Duration.Minutes())
							}
							return ws
						}
						return ""
					}(),
			),
			"\n(Press 'esc' or 'q' to return)",
		))
	}
	return ""
}

func (m model) viewListView() string {
	return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("TermiDone - List View"),
		m.list.View(),
		statusMessageStyle(m.statusMessage),
		helpStyle.Render("n: new | d: del | c: complete | enter: view | ?: help | q: quit | tab: kanban"),
	))
}

func (m model) viewKanbanView() string {
	// Filter tasks by column
	todoTasks := []Task{}
	inProgressTasks := []Task{}
	doneTasks := []Task{}

	for _, task := range m.tasks {
		if task.Completed {
			doneTasks = append(doneTasks, task)
			continue
		}
		switch task.Column {
		case ColumnTodo:
			todoTasks = append(todoTasks, task)
		case ColumnInProgress:
			inProgressTasks = append(inProgressTasks, task)
		}
	}

	// Render columns
	var todoContent, inProgressContent, doneContent string

	// Render TODO column
	for i, task := range todoTasks {
		style := taskStyle
		if m.selectedColumn == ColumnTodo && i == m.selectedTaskIndex[ColumnTodo] {
			style = style.Copy().BorderForeground(lipgloss.Color("205"))
		}
		todoContent += style.Render(task.Title()) + "\n"
	}

	// Render In Progress column
	for i, task := range inProgressTasks {
		style := taskStyle
		if m.selectedColumn == ColumnInProgress && i == m.selectedTaskIndex[ColumnInProgress] {
			style = style.Copy().BorderForeground(lipgloss.Color("205"))
		}
		inProgressContent += style.Render(task.Title()) + "\n"
	}

	// Render Done column
	for i, task := range doneTasks {
		style := completedTaskStyle
		if m.selectedColumn == ColumnDone && i == m.selectedTaskIndex[ColumnDone] {
			style = style.Copy().BorderForeground(lipgloss.Color("205"))
		}
		doneContent += style.Render(task.Title()) + "\n"
	}

	todoColumn := columnStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		"TODO",
		todoContent,
	))
	inProgressColumn := columnStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		"IN PROGRESS",
		inProgressContent,
	))
	doneColumn := columnStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		"DONE",
		doneContent,
	))

	switch m.selectedColumn {
	case ColumnTodo:
		todoColumn = focusedColumnStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			"TODO",
			todoContent,
		))
	case ColumnInProgress:
		inProgressColumn = focusedColumnStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			"IN PROGRESS",
			inProgressContent,
		))
	case ColumnDone:
		doneColumn = focusedColumnStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			"DONE",
			doneContent,
		))
	}

	return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("TermiDone - Kanban View"),
		lipgloss.JoinHorizontal(lipgloss.Top, todoColumn, inProgressColumn, doneColumn),
		statusMessageStyle(m.statusMessage),
		helpStyle.Render("q: quit | tab: list | ←/→: navigate columns | ↑/↓: navigate tasks | enter: move task"),
	))
}

func (m *model) helpView() string {
	var builder strings.Builder
	keybindings := [][2]string{
		{"n", "New task"},
		{"d", "Delete task"},
		{"c", "Complete task"},
		{"t", "Start timer for selected task"},
		{"r", "Generate report"},
		{"enter", "View task details / Move task (Kanban)"},
		{"j/k, ↑/↓", "Navigate list / Navigate tasks (Kanban)"},
		{"←/→", "Navigate columns (Kanban)"},
		{"tab", "Toggle List/Kanban view"},
		{"esc", "Go back / cancel"},
		{"q", "Quit (from main view)"},
		{"?", "Toggle this help view"},
	}

	for _, kb := range keybindings {
		builder.WriteString(helpKeyStyle.Render(kb[0]) + ": " + helpDescStyle.Render(kb[1]) + "\n")
	}

	return builder.String()
}

// formatDuration formats a duration into HH:MM:SS
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// generateReport creates a Markdown report of completed tasks
func (m *model) generateReport() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		m.statusMessage = fmt.Sprintf("Error getting home directory: %v", err)
		return
	}
	configPath := fmt.Sprintf("%s/.config/termidone", homeDir)
	reportPath := fmt.Sprintf("%s/work_report_%s.md", configPath, time.Now().Format("2006-01-02"))

	reportContent := fmt.Sprintf("# Work Report for %s\n\n", time.Now().Format("January 02, 2006"))
	reportContent += "## Completed Tasks Today:\n\n"

	hasCompletedTasks := false
	for _, task := range m.tasks {
		if task.Completed && task.CompletedAt.Year() == time.Now().Year() &&
			task.CompletedAt.Month() == time.Now().Month() &&
			task.CompletedAt.Day() == time.Now().Day() {
			reportContent += fmt.Sprintf("- [x] %s (Completed at %s)\n", task.TaskTitle, task.CompletedAt.Format("15:04"))
			for _, session := range task.WorkSessions {
				reportContent += fmt.Sprintf("  - Work Session: %s - %s (Duration: %s)\n",
					session.Start.Format("15:04"), session.End.Format("15:04"), formatDuration(session.Duration))
			}
			hasCompletedTasks = true
		}
	}

	if !hasCompletedTasks {
		reportContent += "No tasks completed today.\n"
	}

	err = ioutil.WriteFile(reportPath, []byte(reportContent), 0644)
	if err != nil {
		m.statusMessage = fmt.Sprintf("Error writing report file: %v", err)
	} else {
		m.statusMessage = fmt.Sprintf("Report generated: %s", reportPath)
	}
}

// TimerTickMsg is a custom message for timer updates
type TimerTickMsg time.Time

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}