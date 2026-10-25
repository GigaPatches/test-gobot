// Package plugin defines the contract that gobot command plugins implement.
//
// The package is deliberately dependency-free: internal/bot depends on it for
// the Command and Plugin types, feature packages depend on it to describe
// themselves, and neither imports the other. That keeps the dependency graph
// acyclic; main.go is what wires concrete plugins into a running bot.
package plugin

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

// Plugin is a group of commands that share state and a theme.
//
// Implementations are expected to be cheap to construct and independent of each
// other: every New call should hand back a freshly initialised instance, so two
// bots (or two tests) never share mutable state through a package-level
// singleton.
type Plugin interface {
	// Name identifies the plugin in diagnostics and help output.
	Name() string
	// Description is a one-line summary of what the plugin offers.
	Description() string
	// Commands returns the commands the plugin contributes. Names must be
	// unique within a plugin, and should not collide with other plugins or
	// with the bot's built-in commands.
	Commands() []*Command
}
