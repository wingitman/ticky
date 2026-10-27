# Backlog

Issues found during the October 2026 review, deferred behind the Windows /
Command Prompt work. Ordered by priority. Line numbers refer to the code at the
time of the review.

1. **Timer loops.** Keys in the timer screen start extra `tickCmd` chains
   (`internal/app/model.go` timer key handling), and every tick starts another
   `pollSessionCmd` chain, so the countdown speeds up and session reads pile
   up. Keep exactly one tick chain and one poll chain.
2. **Break prompt loses its task.** When focus ends the session is deleted, and
   a queued poll then sets `activeTaskIdx = -1`; Complete/Abandon do nothing and
   Start break saves a session with an empty TaskID.
3. **Save-before-quit ordering.** `tea.Batch(save…, tea.Quit)` can quit before
   writes finish. Use `tea.Sequence`, and pass copies of the store/session to
   save commands so goroutines don't race with `Update`.
4. **Active task by ID.** Replace `activeTaskIdx` with the task ID; deleting an
   earlier task currently retargets it, and `view.go` indexes it unchecked.
5. **Input rules (AGENTS.md).**
   - The `new` key is forwarded into the new-group input.
   - While typing a group name, `d`/`q`/`n`/arrows still act as commands.
   - The close key is checked before text inputs, so `q` can't be typed.
   - No confirmation for group delete, Stop, or Abandon.
   - Ctrl+C isn't handled.
6. **Watcher checks.** Before firing, confirm `session.IsDue` and
   `WatchPID == os.Getpid()`. Check a PID is still ticky before killing it.
   On Unix the relaunched TUI runs in the watcher's background process group.
7. **Desktop app.** `desktop.go` launches the watcher with `ticky-desktop`
   (which ignores `--watch`), never records/kills it, and ignores save errors.
   Use the sibling `ticky(.exe)`.
8. **Small terminals.** `strings.Repeat(…, m.width-2)` and `runes[:m.width-9]`
   panic at tiny widths; `visibleTaskRows` reserves too few rows; the group list
   and theme picker don't scroll; mouse row maths is off by one.
9. **Editor.** Split `$EDITOR` into command + args, prefer `VISUAL`, and reload
   config even when the editor exits non-zero.
10. **Key hints.** Hard-coded hints in `view.go`, `update_view.go`, `update.go`
    and `ctrl+p` should come from the keymap.
11. **Config reload.** Reloading config after every task save resets
    `timeFormatIdx`.
12. **Report times.** Sessions that ended while ticky was closed use
    `time.Now()` instead of `sess.EndTime`; break extend/duration default to 0;
    a `time.Time` `!=` comparison always differs after decoding.
13. **Housekeeping.** Stop committing `bin/` (add it to `.gitignore`); remove
    dead code (`groupOffset`, `newTOMLEncoder`, `decodeTOMLFile`,
    `ValidateThemeFile`, `TotalPauseTime`); fix byte-vs-rune comparison in
    `fuzzyMatch`; limit the Ctrl+P `go run` fallback to the ticky repo.
14. **Optional Windows extras.** Clink prompt status for Command Prompt, a
    configurable install directory for AppLocker-restricted machines, code
    signing, `install.cmd --build-all`, and `releases/windows/ticky-desktop.exe`
    from `make build-all`.
