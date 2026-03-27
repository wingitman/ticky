# ticky

A pomodoro-based focus timer and task scheduler for the terminal. Start a task, let ticky count down in the background, and get notified when it's time to take a break — without losing your place in whatever you were working on.

Supports unconventional time formats: T-shirt sizes, story points, and more.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss).

> Made by [delbysoft](https://github.com/wingitman)

---

## Features

- **Pomodoro timer** — focus sessions in 5-minute increments, followed by a configurable break
- **Background timer** — ticky exits after you start a task so you can get to work; it relaunches automatically when the timer fires
- **Task list** — create tasks with a name, focus duration, and break duration
- **Task groups** — organise tasks into named groups; group headers show the total focus time
- **Pause with notes** — pause the timer at any time and record the reason; interrupts are tracked per task
- **Stop and reset** — cancel a running timer and return the task to its unstarted state
- **Break prompt** — full-screen overlay when focus ends: extend, start break, complete, or abandon
- **Reports** — compare expected vs actual time, see interrupt logs, understand where time goes
- **Completed archive** — browse finished tasks with actual vs expected durations
- **Time formats** — cycle through minutes, seconds, HH:MM, T-shirt sizes, and story points
- **Shell prompt integration** — show the active task and countdown in your terminal prompt via `ticky --status`
- **Configurable keybinds** — every key is remappable in `ticky.toml`
- **Cross-platform** — Linux, macOS, Windows

---

## Requirements

- Go 1.21+ (to build from source)
- A terminal with colour support

---

## Install

### Windows

No `make` or Unix tools required — only [Go](https://go.dev/dl/).

```powershell
git clone https://github.com/wingitman/ticky.git
cd ticky
.\install.ps1
```

This builds the binary, installs it to `%LOCALAPPDATA%\Programs\ticky\`, adds that directory to your user PATH via the registry (no admin required), and automatically sets up the shell prompt integration for your active prompt system (starship or PowerShell `$PROFILE`).

> **Execution policy:** if you get a script-blocked error, run this once:
> ```powershell
> Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
> ```

### macOS / Linux

Requires `make` and Go.

```bash
git clone https://github.com/wingitman/ticky.git
cd ticky
make install
```

This builds the binary, copies it to `~/.local/bin/ticky`, and automatically sets up the shell prompt integration for your active prompt system (starship, bash, zsh, or fish). Make sure `~/.local/bin` is on your `PATH`.

After installing, **enable the overlay** in `ticky.toml` (press `o` inside ticky to open it):

```toml
[display]
show_task_name = true
show_time_left = true
```

Then reload your shell (`source ~/.bashrc`, `exec zsh`, `source ~/.config/fish/config.fish`, etc.) and start a task — your prompt will show the active task and remaining time automatically.

---

## Uninstall

### Windows

```powershell
.\uninstall.ps1
```

Removes the binary, removes it from your user PATH, and removes the ticky prompt integration from your starship config or PowerShell `$PROFILE`.

Your config and task data are left in place — delete `%APPDATA%\Roaming\delbysoft\` manually if you want a full clean uninstall.

### macOS / Linux

```bash
make uninstall
```

Removes the binary from `~/.local/bin` and removes the ticky prompt integration from whichever shell config was patched during install. Your config and task data at `~/.config/delbysoft/` are left untouched.

---

## Usage

### Starting ticky

```bash
ticky
```

### Task list

The main view. Navigate with `↑`/`↓`, press `enter` to start the selected task.

When a task is started, ticky exits so you can get to work. The timer runs in the background. When it fires, ticky relaunches automatically and shows the break prompt.

If you reopen ticky while a timer is running, the task list shows the active task with a `▶` indicator and remaining time. Press `enter` on it to return to the timer view, `p` to pause it, or `x` to stop and reset it entirely.

### Task groups

Press `g` to open the group list. Create a group, then assign tasks to it via the task editor. Group headers in the task list show the total focus time for all tasks in the group.

### Pausing

Press `p` to pause — from the timer screen or the task list when the active task is selected. A prompt asks why you're pausing. The reason is recorded as an interrupt note on the task, visible in reports.

Press `esc` to resume without recording a reason.

### Stopping

Press `x` to stop the timer entirely. This cancels the session and resets the task back to its unstarted state — `StartedAt`, `EndedAt`, and interrupt notes are all cleared. The task stays in your active list and can be started again from scratch.

This is different from abandoning (`a` in the break prompt), which marks the task as abandoned and moves it out of the active list.

### Break prompt

When a focus session ends, ticky shows a full-screen break prompt:

| Key | Action |
|-----|--------|
| `e` | Extend focus by +5 minutes |
| `b` | Start the break timer |
| `c` | Mark the task as complete |
| `a` | Abandon the task |

### Reports

Press `r` to open the report view. It shows every finished task (completed or abandoned) with:

- Expected duration (the focus time you set)
- Actual duration (start to end wall time)
- Delta (how much you over or under ran)
- Interrupt count

Interrupt notes are listed below the table so you can see common reasons your focus was broken.

---

## Shell prompt integration

When a task timer is running in the background, `ticky --status` prints a one-line status string:

```
▶ Write unit tests  ⏱ 18:42
```

It prints nothing when no session is active, so it's safe to embed in your prompt unconditionally.

### Automatic setup (recommended)

`make install` / `.\install.ps1` handles this automatically. It patches your tmux config (if found) for live per-second updates, and your shell prompt config as a fallback. After installing, enable the display options in `ticky.toml` (press `o` inside ticky):

```toml
[display]
show_task_name = true
show_time_left = true
```

Then reload your shell (and run `tmux source-file ~/.config/tmux/tmux.conf` if in tmux) and start a task.

### Manual setup

If you installed the binary without using the installer, or need to set it up yourself:

**Step 1** — enable the display options in `ticky.toml` as above.

**Step 2** — add `ticky --status` to your environment.

#### tmux — live updates every second (recommended)

The tmux status bar re-runs `#(ticky --status)` every second and updates in place — no keypresses required.

Add to `~/.config/tmux/tmux.conf` or `~/.tmux.conf`:

```tmux
# ticky shell integration
set -g status-interval 1
set -g status-right-length 120
set -g status-right "#(ticky --status 2>/dev/null)  #[fg=blue]#{?client_prefix,PREFIX ,}#[fg=brightblack]#h "
```

Adjust the `status-right` format to match your existing theme, then reload:

```bash
tmux source-file ~/.config/tmux/tmux.conf
```

The status appears when a timer is running and disappears automatically when idle.

---

**Shell prompt fallbacks** — update on each Enter keypress (useful when not in tmux):

#### bash

Add to `~/.bashrc`:

```bash
# ticky shell integration
export PS1='$(ticky --status && echo " ")'"$PS1"
```

#### zsh

Add to `~/.zshrc`:

```zsh
# ticky shell integration
RPROMPT='$(ticky --status)'
```

#### fish

Add to `~/.config/fish/config.fish`:

```fish
# ticky shell integration
function fish_right_prompt
    ticky --status
end
```

#### PowerShell

Add to `$PROFILE`:

```powershell
# ticky shell integration
function prompt {
    $s = ticky --status 2>$null
    if ($s) { Write-Host "$s " -NoNewline -ForegroundColor Blue }
    "PS $(Get-Location)> "
}
```

> **Note:** `overlay_corner` in `ticky.toml` controls corner placement inside the ticky TUI only. For the shell prompt, use left-side prompts (`PS1`, `fish_prompt`) for left corners and right-side prompts (`RPROMPT`, `fish_right_prompt`) for right corners.

---

## Neovim integration

If you use Neovim, see [ticky.nvim](https://github.com/wingitman/ticky.nvim) for native editor integration — including notifications when your focus session ends, directly inside Neovim.

For shell scripts and editor plugins, `ticky --check` provides a polling interface:

```bash
$ ticky --check   # timer running
running: 18:42 remaining in focus for "Write unit tests"
ends_at: 10:25:00
nvim_socket: /run/user/1000/nvim.12345.0

$ ticky --check   # timer just fired
due: focus session complete for "Write unit tests" — open ticky for your break

$ ticky --check   # nothing running
idle: no active session
```

Exit codes: `0` = due (break prompt needed), `1` = running, `2` = idle.

---

## Keybinds

All keybinds are configurable in `ticky.toml`. These are the defaults.

### Task list

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection up / down |
| `n` | New task |
| `e` | Edit selected task |
| `d` | Delete selected task |
| `enter` | Start selected task / resume active task |
| `p` | Pause active timer (prompts for reason) |
| `x` | Stop active timer and reset task to unstarted |
| `g` | Open group list |
| `r` | Report view |
| `h` | Completed tasks |
| `f` | Cycle time format |
| `o` | Open `ticky.toml` in `$EDITOR` |
| `q` / `esc` | Quit (active timer keeps running in background) |

### Timer screen

| Key | Action |
|-----|--------|
| `p` | Pause (prompts for reason) |
| `x` | Stop and reset task |
| `q` / `esc` | Back to task list (timer keeps running) |

### Break prompt

| Key | Action |
|-----|--------|
| `e` | Extend focus +5m |
| `b` | Start break |
| `c` | Complete task |
| `a` | Abandon task |

### Task editor

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Next / previous field |
| `enter` | Save |
| `esc` | Cancel |

---

## Time formats

Cycle through formats with `f`.

| Format | Example (25 min) | Notes |
|--------|-----------------|-------|
| `minutes` | `25m` | Default |
| `seconds` | `1500s` | |
| `hhmm` | `00:25` | |
| `tshirt` | `S` | XS ≤15m · S ≤30m · M ≤60m · L ≤2h · XL ≤4h · XXL >4h |
| `points` | `2` | 1=≤15m · 2=≤30m · 3=≤60m · 5=≤2h · 8=≤4h · 13=>4h |

---

## Configuration

The config file is created automatically on first launch. Press `o` inside ticky to open it in your editor.

| OS | Path |
|----|------|
| Linux | `~/.config/delbysoft/ticky.toml` |
| macOS | `~/Library/Application Support/delbysoft/ticky.toml` |
| Windows | `%APPDATA%\Roaming\delbysoft\ticky.toml` |

Task data is stored as `tasks.toml` in the same directory.

### Full default config

```toml
# ticky configuration file

[keybinds]
up        = "up"      # move selection up
down      = "down"    # move selection down
edit      = "e"       # edit selected task
confirm   = "enter"   # save / confirm
start     = "enter"   # start selected task timer
close     = "q"       # quit ticky
format    = "f"       # cycle time format
options   = "o"       # open this config file in your editor
pause     = "p"       # pause the running timer
stop      = "x"       # stop the running timer and reset the task
new       = "n"       # create a new task
delete    = "d"       # delete selected task
group     = "g"       # open group list
report    = "r"       # open report view
completed = "h"       # view completed tasks

[display]
# How task durations are displayed.
# Options: minutes | seconds | hhmm | tshirt | points
time_format = "minutes"

# Show the active task name in a terminal corner when a timer is running.
# Add 'ticky --status' to your shell prompt to display this outside ticky.
show_task_name = false

# Show the remaining timer time in a terminal corner when a timer is running.
show_time_left = false

# Which corner to render the status overlay in (inside the ticky TUI).
# Options: top-left | top-right | bottom-left | bottom-right
overlay_corner = "top-right"
```

### Vim-style keybinds

```toml
[keybinds]
up   = "k"
down = "j"
```

---

## Building from source

### macOS / Linux

```bash
make build    # → bin/ticky
make install  # build + install to ~/.local/bin
make test     # run unit tests
make clean
make uninstall
```

### Windows (PowerShell)

```powershell
go build -ldflags='-s -w' -o bin\ticky.exe .   # build only
.\install.ps1                                    # build + install
.\uninstall.ps1                                  # uninstall
```

### Cross-compile

```bash
GOOS=darwin  GOARCH=arm64 go build -o bin/ticky-macos-arm64 .
GOOS=linux   GOARCH=amd64 go build -o bin/ticky-linux-amd64 .
GOOS=windows GOARCH=amd64 go build -o bin/ticky-windows.exe .
```

---

## Support

<a href='https://ko-fi.com/W7W21WP5L7' target='_blank'><img height='36' style='border:0px;height:36px;' src='https://storage.ko-fi.com/cdn/kofi4.png?v=6' border='0' alt='Buy Me a Coffee at ko-fi.com' /></a>

---

## License

MIT — see [LICENSE](LICENSE).

Copyright (c) 2026 [delbysoft](https://github.com/wingitman)
