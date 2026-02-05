package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/logging"
	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/loop"
)

const (
	defaultHost        = "::"
	defaultPort        = "2222"
	defaultHostKeyPath = "/app/keys/host_key"
)

// Global game server - shared by all SSH clients
var gameServer *loop.Server
var serverOnce sync.Once

func main() {
	host := getEnv("SSH_HOST", defaultHost)
	port := getEnv("SSH_PORT", defaultPort)
	hostKeyPath := getEnv("SSH_HOST_KEY", defaultHostKeyPath)
	workingDir, workErr := os.Getwd()
	if workErr != nil {
		log.Printf("Failed to get working directory: %v", workErr)
	}
	log.Printf("SSH config: host=%s port=%s hostKeyPath=%s workingDir=%s", host, port, hostKeyPath, workingDir)

	// Initialize and start the shared game server
	serverOnce.Do(func() {
		gameServer = loop.NewServer()
		go gameServer.Run()
		log.Println("Game server started")
	})

	opts := []ssh.Option{
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithMiddleware(
			gameMiddleware,
			activeterm.Middleware(),
			logging.Middleware(),
		),
	}

	if hostKeyPath != "" {
		opts = append(opts, wish.WithHostKeyPath(hostKeyPath))
	}

	s, err := wish.NewServer(opts...)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Starting SSH server on %s:%s", host, port)
	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down server...")

	// Stop the game server
	if gameServer != nil {
		gameServer.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
}

// gameMiddleware handles SSH sessions and runs the game client.
func gameMiddleware(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		pty, winCh, ok := sess.Pty()
		if !ok {
			fmt.Fprintln(sess, "Error: PTY required. Please connect with: ssh -t user@host")
			return
		}

		log.Printf("New game session: user=%s, terminal=%s, size=%dx%d",
			sess.User(), pty.Term, pty.Window.Width, pty.Window.Height)

		// Create a terminal size tracker that updates on window changes
		sizeTracker := newSizeTracker(pty.Window.Width, pty.Window.Height)

		// Listen for window size changes in a goroutine
		go func() {
			for win := range winCh {
				sizeTracker.update(win.Width, win.Height)
			}
		}()

		reader := bufio.NewReader(sess)
		clientOpts := loop.ClientOptions{
			TermSizeFunc: sizeTracker.getSize,
		}

		// Create a new client connected to the shared game server
		client := loop.NewClient(gameServer, reader, sess, clientOpts)
		if err := client.Run(); err != nil {
			log.Printf("Game error for %s: %v", sess.User(), err)
		}

		log.Printf("Session ended: user=%s", sess.User())
		next(sess)
	}
}

// sizeTracker tracks terminal size from SSH window change events.
type sizeTracker struct {
	mu     sync.RWMutex
	width  int
	height int
}

func newSizeTracker(width, height int) *sizeTracker {
	return &sizeTracker{width: width, height: height}
}

func (s *sizeTracker) update(width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.width = width
	s.height = height
}

func (s *sizeTracker) getSize() (int, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.width, s.height, nil
}

// Ensure sizeTracker.getSize satisfies draw.TermSizeFunc
var _ draw.TermSizeFunc = (*sizeTracker)(nil).getSize

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
