# AGENTS.md

Guidance for agents working in this repository.

## Overview

`gobot` is a CLI chatbot written in Go. Module name is `gobot` (Go 1.27.1). The
root `main.go` is the only binary entrypoint; the core lives in `internal/bot`
and features ship as plugins under `internal/plugins`.

## Commands

Use [go-task](https://taskfile.dev) (`Taskfile.yml`). The `task` binary lives at
`~/go/bin/task` and may not be on `PATH` — call it by full path if `task` is not found.

- `task check` — gofmt check + `go vet` + tests. Run this before considering work done.
- `task build` — build to `bin/gobot` (cached; skips if sources unchanged).
- `task run` — start the interactive REPL.
- `task cover` — tests with coverage summary.
- `task clean` — remove `bin/`, `coverage.out`.

Single test: `go test ./internal/bot -run TestReplyPing`, or
`go test ./internal/plugins/calc -run TestArithmetic`.

## Architecture

- `internal/bot/bot.go` defines `Bot`, the command registry plus REPL loop.
  `bot.Command` is an alias for `plugin.Command`, so the registry and the plugins
  share one type without importing each other.
- `internal/plugin/plugin.go` is the plugin contract: `Command{Name, Description,
  Handler}` and `Plugin{Name, Description, Commands}`. It imports nothing, which
  keeps the graph acyclic — `bot` and the feature packages both depend on it, and
  only `main.go` wires them together.
- `internal/plugins/plugins.go` holds `All`, the ordered list of plugins a bot
  loads. Adding a plugin means adding one line there.
- `internal/plugins/calc/` is the reference plugin: a scientific calculator with
  its own tokenizer and recursive-descent parser (`eval.go`), its function table
  (`funcs.go`), and its stateful `calc` command (`calc.go`).
- `internal/plugins/units/` is a second, stateless plugin: `convert` resolves
  unit expressions such as `km/h` or `ft^2` (`parse.go`) against a table of units
  that each carry a scale and a dimension vector (`unit.go`), so compound and
  derived units work without special cases.
- `Bot.Reply(line string) string` is pure line-in/response-out with no I/O — put
  testable logic there. `Bot.Run(in, out)` owns all I/O (prompts, `exit`/`quit`,
  EOF handling).
- Built-in commands are registered in `bot.New()` via `b.Register(&Command{...})`;
  plugin commands arrive through `b.Load(p)`. Registering a duplicate name panics
  by design.
- Command names are matched case-insensitively; extra args after the command name
  are passed to `Handler` as `args []string`.

## Conventions

- All Go files must be gofmt-clean; `task check` fails otherwise.
- Tests live next to code (`internal/bot/bot_test.go`) using plain stdlib
  `testing`, one focused test per behavior.
- Keep the REPL transport-agnostic: `Run` takes `io.Reader`/`io.Writer` so future
  transports (network, web) can reuse `Bot.Reply`.
- A plugin's `New()` returns an independent instance holding its own state, so
  two bots never share mutable data. Guard that state with a mutex if a handler
  could be called concurrently, and keep the pure parts (the calculator's parser,
  for instance) free of locking so they stay easy to test.
- Plugins report user-facing failures as a returned `error: ...` string rather
  than panicking; only programming errors (duplicate command names) panic.
