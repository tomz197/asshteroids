// Package loop provides the client-server game loop and state management.
package loop

import (
	"bufio"
	"context"
	"io"

	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/loop/client"
	"github.com/tomz197/asteroids/internal/loop/server"
)

// RunClientServer starts the game in client-server mode.
// Creates a local server in a background goroutine and runs a single client
// in the calling goroutine. Blocks until the client disconnects.
// Use this for standalone/single-player mode.
func RunClientServer(r *bufio.Reader, w io.Writer, opts client.ClientOptions) error {
	if opts.TermSizeFunc == nil {
		opts.TermSizeFunc = draw.DefaultTermSizeFunc
	}

	// Create and start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := server.NewServer()
	go srv.Run(ctx)

	// Create and run client
	c := client.NewClient(srv, r, w, opts)
	return c.Run()
}
