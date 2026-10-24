// Package bot implements the chatbot core: a registry of commands and a
// read-eval-print loop that dispatches user input to them.
package bot

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Command is a single chatbot command.
type Command struct {
	// Name is the word the user types to invoke the command.
	Name string
	// Description is a short one-line summary, used for help text.
	Description string
	// Handler produces the bot's response. args holds any words typed
	// after the command name.
	Handler func(args []string) string
}

// Bot holds the registered commands and processes user input.
type Bot struct {
	commands map[string]*Command
}

// New creates a Bot with the default set of commands registered.
func New() *Bot {
	b := &Bot{commands: make(map[string]*Command)}
	b.Register(&Command{
		Name:        "ping",
		Description: "check that the bot is alive",
		Handler: func(args []string) string {
			return "pong"
		},
	})
	return b
}

// Register adds a command to the bot. It panics if the name is already taken,
// since that is always a programming error.
func (b *Bot) Register(cmd *Command) {
	if _, dup := b.commands[cmd.Name]; dup {
		panic("bot: duplicate command " + cmd.Name)
	}
	b.commands[cmd.Name] = cmd
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
