package main

import (
	"fmt"
	"os"

	"gobot/internal/bot"
	"gobot/internal/plugins"
)

func main() {
	b := bot.New(plugins.All...)
	fmt.Println("gobot chatbot — type `help` for commands. Ctrl-D or `exit` to quit.")
	if err := b.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
