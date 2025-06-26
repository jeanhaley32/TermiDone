# TermiDone

**This project was created with the help of the Gemini CLI.**

A terminal-based TODO list and work timer built with Go and the Charmbracelet suite.

## Features

- **TODO List Management:** View, add, complete, and delete tasks.
- **Kanban Board View:** Organize tasks into "To Do", "In Progress", and "Done" columns.
- **Integrated Work Timer:** Start a Pomodoro-style timer for any task.
- **State Persistence:** Your tasks are saved to `~/.config/termidone/tasks.json` and reloaded on startup.
- **Work Reporting:** Generate a Markdown report of completed tasks for the day.

## Installation & Usage

1.  **Prerequisites:** Ensure you have Go installed (version 1.18 or later).

2.  **Build the application:**
    ```bash
    cd ~/TermiDone
    go build -o termidone
    ```

3.  **Run the application:**
    ```bash
    ./termidone
    ```

    *Optional: Move the `termidone` binary to a directory in your system's PATH to run it from anywhere.*
    ```bash
    mv termidone /usr/local/bin/
    ```

## Keybindings

| Key         | Description                       |
|-------------|-----------------------------------|
| `n`         | New task                          |
| `d`         | Delete task                       |
| `c`         | Complete task                     |
| `t`         | Start timer for selected task     |
| `r`         | Generate report                   |
| `enter`     | View task details / Move task (Kanban)    |
| `j`/`k`, `↑`/`↓` | Navigate list / Navigate tasks (Kanban)   |
| `tab`       | Toggle List/Kanban view           |
| `←`/`→`     | Navigate columns (Kanban)         |
| `u`         | Uncomplete task (Kanban)          |
| `esc`       | Go back / cancel                  |
| `q`         | Quit (from main view)             |
| `?`         | Toggle this help view             |