# Ticky Architecture

Ticky follows Bubble Tea's model-update-view loop.

## Boundaries

- `internal/storage` is the task/group repository and contains pure collection
  operations such as grouping, filtering, and totals.
- `internal/config` loads and validates user preferences. The app refreshes its
  in-memory configuration after an external editor exits.
- `internal/app` coordinates screens and commands. Rendering should consume
  view projections and never reorder the persisted store.
- `internal/ui` is the delbysoft visual language shared by screens.
- `internal/desktop` is an optional Ebiten frontend. It runs as a separate
  `ticky-desktop` process and uses the same storage/session files as the TUI.

## List Rendering

Task rows are projected into display rows containing group headers and tasks.
The projection owns display ordering, row-to-task mapping, and visible-height
calculations. Stored task order remains unchanged.

## Safety

Delete actions use a confirmation mode. The confirmation prompt identifies the
target and defaults to cancellation. Active timer tasks cannot be deleted.

## Responsive Rendering

Every rendered line is width-clamped and every screen limits its content to the
available height. Hints are context-sensitive and may collapse to a smaller
global set on narrow terminals.

## Desktop Frontend

The TUI launches the desktop process detached from the terminal. The desktop
frontend can show a full task view or compact widget and delegates timer state
to the persisted session so either frontend can remain open independently.
