package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/tomz197/asteroids/internal/loop"
	"golang.org/x/term"
)

func main() {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to enable raw mode: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = term.Restore(fd, oldState)
	}()

	reader := bufio.NewReader(os.Stdin)
	if err := loop.Run(reader, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "game error: %v\n", err)
		os.Exit(1)
	}
}
