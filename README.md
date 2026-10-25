# gobot

A small CLI chatbot written in Go, with a scientific calculator and a unit
converter as plugins. No third-party dependencies — standard library only.

## Quick start

Requires Go 1.27 or newer (see `go.mod`). [go-task](https://taskfile.dev) is
convenient but optional — every task is a plain `go` command.

```sh
task run          # or: go run .
```

```
gobot chatbot — type `help` for commands. Ctrl-D or `exit` to quit.
> ping
pong
> calc sqrt(2) * pi
4.44288293816
> convert 10 km to mi
10 km = 6.21371192237 mi
> help
calc — evaluate a math expression, e.g. `calc sqrt(2) * pi`
convert — convert an amount between units, e.g. `convert 10 km to mi`
help — list commands, or describe one: `help <command>`
ping — check that the bot is alive
units — same as `convert`
```

Build a binary with `task build`, which writes `bin/gobot`.

## Commands

Command names are case-insensitive. `help <command>` describes one command or
plugin; `exit`, `quit` or Ctrl-D leaves.

### `calc` — scientific calculator

```
calc 2^3^2              512                   ^ is right-associative, ** works too
calc 0.1 + 0.2          0.3
calc 5!                 120
calc 2pi                6.28318530718         implicit multiplication
calc log(8, 2)          3
calc x = 3 + 4          7                     user variables, and `ans`
calc x * 2              14
calc mode deg           mode: deg             deg/rad angle mode
calc sin(90)            1
```

`calc vars`, `calc hist` and `calc clear` show and reset state; `calc help`
documents every function.

Numbers can carry units, and units survive arithmetic:

```
calc 10 km to mi        6.21371192237 mi      `to` (or `->`) converts
calc 100 km / 2 h       50 km/h
calc 5 m + 3 ft         5.9144 m              additions keep the left unit
calc 2 kg * 3 m/s^2     6 kg*m/s^2
calc (2 m)^2            4 m^2
calc sqrt(16 m^2)       4 m
calc sin(45 deg)        0.707106781187        an angle needs no mode
calc 32 F to C          0 C
```

### `convert` (also `units`) — unit converter

```
convert 10 km to mi             10 km = 6.21371192237 mi
convert 100 km/h to mph         100 km/h = 62.1371192237 mph
convert 32 F to C               32 F = 0 C
convert 2 ft^2 to in^2          2 ft^2 = 288 in^2
convert 1 GiB to MB             1 GiB = 1073.741824 MB
convert 1 stone to kg           1 stone = 6.35029318 kg
```

Compound units (`km/h`, `m/s^2`, `lb/ft^3`), SI and binary prefixes, spelled-out
names and plurals all work. `convert units` lists every known unit by family,
and `convert help` explains the syntax.

## Development

Use [go-task](https://taskfile.dev) (`Taskfile.yml`):

```sh
task check    # gofmt check + go vet + tests
task build    # build to bin/gobot
task cover    # tests with a coverage summary
task clean    # remove bin/ and coverage.out
```

A single test:

```sh
go test ./internal/bot -run TestReplyPing
```

## Layout

```
main.go                      wires the plugins into a bot and runs the REPL
internal/bot/                command registry and the REPL loop
internal/plugin/             the plugin contract, imports nothing
internal/plugins/            the list of plugins a bot loads
internal/plugins/calc/       calculator: tokenizer, parser, functions, quantities
internal/plugins/units/      the `convert` command, a thin wrapper over the engine
internal/units/              the measurement engine shared by both plugins
```

`Bot.Reply(line)` is pure line-in/response-out with no I/O, which is where
testable logic goes; `Bot.Run(in, out)` owns all I/O and takes `io.Reader` and
`io.Writer` so another transport could reuse it.

## Adding a plugin

Implement the small contract in `internal/plugin`:

```go
type Command struct {
	Name        string
	Description string
	Handler     func(args []string) string
}

type Plugin interface {
	Name() string
	Description() string
	Commands() []*Command
}
```

then add one line to `All` in `internal/plugins/plugins.go`. Plugins never
import each other: knowledge two of them want moves to a plain library package
that both import, which is how `internal/units` came to be.
