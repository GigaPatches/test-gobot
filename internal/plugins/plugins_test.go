package plugins

import (
	"strings"
	"testing"

	"gobot/internal/bot"
)

func TestAllPluginsLoadIntoABot(t *testing.T) {
	b := bot.New(All...)
	if got := b.Reply("ping"); got != "pong" {
		t.Errorf("ping = %q, want pong", got)
	}
	if got := b.Reply("calc 2+2"); got != "4" {
		t.Errorf("calc 2+2 = %q, want 4", got)
	}
	help := b.Reply("help")
	for _, p := range All {
		for _, cmd := range p.Commands() {
			if !strings.Contains(help, cmd.Name) {
				t.Errorf("help does not list %s from plugin %s:\n%s", cmd.Name, p.Name(), help)
			}
		}
	}
}

// TestPluginCommandsAreWellFormed guards the registry contract: every plugin
// describes itself and every command name is used only once.
func TestPluginCommandsAreWellFormed(t *testing.T) {
	owner := map[string]string{}
	for _, p := range All {
		if p.Name() == "" {
			t.Error("a plugin has an empty Name")
		}
		if p.Description() == "" {
			t.Errorf("plugin %s has an empty Description", p.Name())
		}
		for _, cmd := range p.Commands() {
			if cmd.Name == "" {
				t.Errorf("plugin %s has a command with no Name", p.Name())
				continue
			}
			if cmd.Description == "" {
				t.Errorf("command %s has an empty Description", cmd.Name)
			}
			if cmd.Handler == nil {
				t.Errorf("command %s has no Handler", cmd.Name)
			}
			if prev, dup := owner[cmd.Name]; dup {
				t.Errorf("command %q is registered by both %s and %s", cmd.Name, prev, p.Name())
			}
			owner[cmd.Name] = p.Name()
		}
	}
}
