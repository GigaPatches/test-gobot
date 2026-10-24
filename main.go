package main

import (
	"fmt"
	"os"

	"gobot/internal/bot"
)

func main() {
	b := bot.New()
	fmt.Println("gobot chatbot — type a command (e.g. `ping`). Ctrl-D or `exit` to quit.")
	if err := b.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
