package bot

import "testing"

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
