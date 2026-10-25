package bot

import (
	"strings"
	"testing"

	"gobot/internal/plugin"
)

// fakePlugin is a minimal plugin for exercising the registry without pulling a
// real feature package into the bot's tests.
type fakePlugin struct {
	name string
	cmds []*plugin.Command
}

func (f fakePlugin) Name() string        { return f.name }
func (f fakePlugin) Description() string { return "test fixture" }
func (f fakePlugin) Commands() []*plugin.Command {
	return f.cmds
}

func newFakePlugin() fakePlugin {
	return fakePlugin{name: "faker", cmds: []*plugin.Command{{
		Name:        "fake",
		Description: "a fake command",
		Handler:     func(args []string) string { return "fake " + strings.Join(args, " ") },
	}}}
}

func TestReplyPing(t *testing.T) {
	b := New()
	if got := b.Reply("ping"); got != "pong" {
		t.Errorf("Reply(\"ping\") = %q, want %q", got, "pong")
	}
}

func TestReplyIsCaseInsensitive(t *testing.T) {
	b := New()
	if got := b.Reply("PING"); got != "pong" {
		t.Errorf("Reply(\"PING\") = %q, want %q", got, "pong")
	}
}

func TestReplyIgnoresExtraArgs(t *testing.T) {
	b := New()
	if got := b.Reply("ping extra args"); got != "pong" {
		t.Errorf("Reply(\"ping extra args\") = %q, want %q", got, "pong")
	}
}

func TestReplyUnknownCommand(t *testing.T) {
	b := New()
	if got := b.Reply("dance"); got == "" {
		t.Error("Reply(\"dance\") = empty, want an unknown-command message")
	}
}

func TestReplyEmptyLine(t *testing.T) {
	b := New()
	if got := b.Reply("   "); got != "" {
		t.Errorf("Reply(\"   \") = %q, want empty", got)
	}
}

func TestPluginCommandsAreRegistered(t *testing.T) {
	b := New(newFakePlugin())
	if got := b.Reply("fake hi"); got != "fake hi" {
		t.Errorf("Reply(\"fake hi\") = %q, want %q", got, "fake hi")
	}
	if got := b.Reply("FAKE"); got != "fake " {
		t.Errorf("Reply(\"FAKE\") = %q, want %q", got, "fake ")
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	b := New(newFakePlugin())
	got := b.Reply("help")
	lines := strings.Split(got, "\n")
	want := []string{
		"fake — a fake command",
		"help — list commands, or describe one: `help <command>`",
		"ping — check that the bot is alive",
	}
	if len(lines) != len(want) {
		t.Fatalf("help listed %d commands, want %d:\n%s", len(lines), len(want), got)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("help line %d = %q, want %q", i+1, lines[i], want[i])
		}
	}
}

func TestHelpDescribesOneCommand(t *testing.T) {
	b := New()
	if got := b.Reply("help ping"); got != "ping — check that the bot is alive" {
		t.Errorf("help ping = %q", got)
	}
	if got := b.Reply("help dance"); got != `unknown command "dance"` {
		t.Errorf("help dance = %q, want an unknown-command message", got)
	}
}

func TestHelpFallsBackToPluginName(t *testing.T) {
	b := New(newFakePlugin())
	if got := b.Reply("help faker"); got != "plugin faker — test fixture" {
		t.Errorf("help faker = %q", got)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Register with a duplicate name did not panic")
		}
	}()
	b := New(newFakePlugin())
	b.Register(&Command{Name: "fake", Description: "impostor", Handler: func([]string) string { return "" }})
}
