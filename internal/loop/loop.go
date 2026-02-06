// Package loop provides the client-server game loop and state management.
package loop

import (
	"bufio"
	"context"
	"io"

	"github.com/tomz197/asteroids/internal/draw"
)

// View resolution - the visible viewport in logical units.
// Actual rendering scales to fit terminal size.
const (
	viewWidth  = 120 // Logical viewport width
	viewHeight = 80  // Logical viewport height (in sub-pixels, so 40 terminal rows)
)

// World dimensions - the total game area (larger than viewport).
// Ship stays centered while the camera follows it.
const (
	worldWidth  = 400 // Total world width
	worldHeight = 300 // Total world height
)

// RunClientServer starts the game in client-server mode.
// Creates a local server in a background goroutine and runs a single client
// in the calling goroutine. Blocks until the client disconnects.
// Use this for standalone/single-player mode.
func RunClientServer(r *bufio.Reader, w io.Writer, opts ClientOptions) error {
	if opts.TermSizeFunc == nil {
		opts.TermSizeFunc = draw.DefaultTermSizeFunc
	}

	// Create and start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewServer()
	go server.Run(ctx)

	// Create and run client
	client := NewClient(server, r, w, opts)
	return client.Run()
}
