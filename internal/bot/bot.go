// Package bot implements the chatbot core: a registry of commands and a
// read-eval-print loop that dispatches user input to them.
package bot

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"gobot/internal/plugin"
)

// Command is a single chatbot command. It is an alias for plugin.Command so the
// registry and the plugins share one type without importing each other.
type Command = plugin.Command

// Bot holds the registered commands and processes user input.
type Bot struct {
	commands map[string]*Command
	plugins  []plugin.Plugin
}

// New creates a Bot with the built-in commands plus every plugin given. Plugins
// are loaded in order, so a later plugin cannot replace an earlier command.
func New(plugins ...plugin.Plugin) *Bot {
	b := &Bot{commands: make(map[string]*Command)}
	b.Register(&Command{
		Name:        "ping",
		Description: "check that the bot is alive",
		Handler: func(args []string) string {
			return "pong"
		},
	})
	b.Register(&Command{
		Name:        "help",
		Description: "list commands, or describe one: `help <command>`",
		Handler:     b.handleHelp,
	})
	for _, p := range plugins {
		b.Load(p)
	}
	return b
}

// Load registers every command a plugin provides and remembers the plugin for
// help output. It panics if a plugin reuses a command name, since that is always
// a programming error.
func (b *Bot) Load(p plugin.Plugin) {
	for _, cmd := range p.Commands() {
		b.Register(cmd)
	}
	b.plugins = append(b.plugins, p)
}

// Register adds a command to the bot. It panics if the name is already taken,
// since that is always a programming error.
func (b *Bot) Register(cmd *Command) {
	if _, dup := b.commands[cmd.Name]; dup {
		panic("bot: duplicate command " + cmd.Name)
	}
	b.commands[cmd.Name] = cmd
}

// handleHelp lists the registered commands, or describes a single command or
// plugin.
func (b *Bot) handleHelp(args []string) string {
	if len(args) > 0 {
		name := strings.ToLower(args[0])
		if cmd, ok := b.commands[name]; ok {
			return fmt.Sprintf("%s — %s", cmd.Name, cmd.Description)
		}
		for _, p := range b.plugins {
			if strings.EqualFold(p.Name(), name) {
				return fmt.Sprintf("plugin %s — %s", p.Name(), p.Description())
			}
		}
		return fmt.Sprintf("unknown command %q", name)
	}
	names := make([]string, 0, len(b.commands))
	for name := range b.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	var list strings.Builder
	for _, name := range names {
		fmt.Fprintf(&list, "%s — %s\n", name, b.commands[name].Description)
	}
	return strings.TrimSuffix(list.String(), "\n")
}

// Reply parses one line of user input and returns the bot's response.
func (b *Bot) Reply(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	name := strings.ToLower(fields[0])
	cmd, ok := b.commands[name]
	if !ok {
		return fmt.Sprintf("unknown command %q", name)
	}
	return cmd.Handler(fields[1:])
}

// Run reads lines from in and writes replies to out until input ends.
func (b *Bot) Run(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "> ")
		if !scanner.Scan() {
			fmt.Fprintln(out)
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		switch strings.ToLower(line) {
		case "":
			continue
		case "exit", "quit":
			fmt.Fprintln(out, "bye")
			return nil
		}
		fmt.Fprintln(out, b.Reply(line))
	}
}
