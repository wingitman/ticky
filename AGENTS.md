# Ticky Development Context

## Project

Ticky is a Go terminal focus timer and task scheduler. It is one of the
delbysoft TUIs and should share their visual language and interaction patterns.

## Architecture

- `main.go` owns CLI modes and Bubble Tea program startup.
- `internal/app` owns the Bubble Tea model, input routing, commands, and views.
- `internal/config` owns TOML defaults, migration, validation, and editor paths.
- `internal/storage` owns task/group persistence and pure store operations.
- `internal/session` owns persisted timer sessions.
- `internal/timer` owns timer state transitions.
- `internal/ui` owns shared colors and Lip Gloss styles.
- `internal/desktop` owns the optional Ebiten frontend and is built with the
  `desktop` build tag as `ticky-desktop`.

Keep persistence and domain operations separate from rendering. Prefer small
pure helpers and projections for list rendering instead of mutating stored
order or embedding business rules in view code.

## UI Conventions

- Use the delbysoft wordmark: white `delby` followed by blue `soft` (`#5865F2`).
- Use the family palette: primary `#7C9EF0`, accent `#F0A47C`, muted `#666688`,
  selection `#2A2A4A`, success `#7CF09C`, and error `#F07C7C`.
- Render context-sensitive hints from the active keymap, never hard-code keys.
- All views must respect the current terminal width and height, including
  narrow terminals and terminals with only a few rows.
- Prefer rounded panels, clear empty states, strong selected-row contrast, and
  full-width dividers.

## Input Rules

- A key that changes mode must not also be forwarded to a newly focused input.
- Destructive actions require explicit confirmation with cancel as the safe
  default.
- Config changes made in `$EDITOR` must be reloaded before the next view.

## Verification

Run `gofmt -w` on changed Go files, then `go test ./...`, `go vet ./...`, and
`go build ./...`. Test rendering behavior at small terminal sizes as well as
normal desktop sizes.

## Dependencies

Use Bubble Tea v2, Bubbles v2, and Lip Gloss v2 for new work. Follow the
upgrade guides when changing v1 APIs rather than adding compatibility wrappers.
The desktop frontend uses Ebiten and must not be pulled into portable
cross-platform CLI builds.
