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
)

// Task represents a single TODO item
type Task struct {
	ID           int           `json:"id"`
	TaskTitle    string        `json:"title"`
	Completed    bool          `json:"completed"`
	CreatedAt    time.Time     `json:"created_at"`
	CompletedAt  time.Time     `json:"completed_at"`
	WorkSessions []WorkSession `json:"work_sessions"`
}

// WorkSession represents a single work session for a task
type WorkSession struct {
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
}

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

// AppState represents the different states of the application
type AppState int

const (
	StateViewTasks AppState = iota
	StateAddTask
	StateTimer
	StateHelp
	StateViewTask
)

// Model represents the application's state
type model struct {
	tasks         []Task
	list          list.Model
	textInput     textinput.Model
	state         AppState
	quitting      bool
	currentTask   *Task // The task currently being worked on
	timer         *time.Timer
	timerDuration time.Duration
	timerStartTime time.Time
	statusMessage string
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
		timerDuration: 25 * time.Minute, // Default Pomodoro duration
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
	case tea.KeyMsg:
		switch m.state {
		case StateViewTasks:
			switch msg.String() {
			case "ctrl+c", "q":
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
			case "ctrl+c", "q", "esc": // Allow quitting/stopping timer
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

	if m.state == StateViewTasks {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.quitting {
		return "Exiting TermiDone. Goodbye!\n"
	}

	switch m.state {
	case StateViewTasks:
		return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("TermiDone"),
			m.list.View(),
			statusMessageStyle(m.statusMessage),
			helpStyle.Render("n: new | d: del | c: complete | enter: view | ?: help | q: quit"),
		))
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

func (m *model) helpView() string {
	var builder strings.Builder
	keybindings := [][2]string{
		{"n", "New task"},
		{"d", "Delete task"},
		{"c", "Complete task"},
		{"t", "Start timer for selected task"},
		{"r", "Generate report"},
		{"enter", "View task details"},
		{"j/k, ↑/↓", "Navigate list"},
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