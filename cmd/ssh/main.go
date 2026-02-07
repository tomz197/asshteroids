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
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/logging"
	"github.com/tomz197/asteroids/internal/config"
	"github.com/tomz197/asteroids/internal/draw"
	"github.com/tomz197/asteroids/internal/loop/client"
	"github.com/tomz197/asteroids/internal/loop/server"
)

const (
	defaultHost        = "::"
	defaultPort        = "2222"
	defaultHostKeyPath = "/app/keys/host_key"
	maxUsernameLength  = 16
)

// Global game server - shared by all SSH clients
var (
	gameServer   *server.Server
	cancelServer context.CancelFunc
	serverOnce   sync.Once
)

func main() {
	if err := config.LoadEnvFile(".env"); err != nil {
		log.Printf("Warning: failed to load .env file: %v", err)
	}

	host := config.GetEnv("SSH_HOST", defaultHost)
	port := config.GetEnv("SSH_PORT", defaultPort)
	hostKeyPath := config.GetEnv("SSH_HOST_KEY", defaultHostKeyPath)
	workingDir, workErr := os.Getwd()
	if workErr != nil {
		log.Printf("Failed to get working directory: %v", workErr)
	}
	log.Printf("SSH config: host=%s port=%s hostKeyPath=%s workingDir=%s", host, port, hostKeyPath, workingDir)

	// Initialize and start the shared game server
	serverOnce.Do(func() {
		var ctx context.Context
		ctx, cancelServer = context.WithCancel(context.Background())
		gameServer = server.NewServer()
		go gameServer.Run(ctx)
		log.Println("Game server started")
	})

	opts := []ssh.Option{
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithMiddleware(
			gameMiddleware,
			activeterm.Middleware(),
			logging.Middleware(),
		),
		// Set TCP_NODELAY to reduce latency for game input
		ssh.WrapConn(func(ctx ssh.Context, conn net.Conn) net.Conn {
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				_ = tcpConn.SetNoDelay(true)
			}
			return conn
		}),
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

	// Gracefully shut down the game server: notify players and wait for them to disconnect
	if gameServer != nil {
		log.Println("Notifying connected players about shutdown...")
		gameServer.Shutdown(15 * time.Second)
		cancelServer()
		log.Println("Game server stopped")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		clientOpts := client.ClientOptions{
			TermSizeFunc: sizeTracker.getSize,
			Username:     sanitizeUsername(sess.User()),
		}

		// Create a new client connected to the shared game server
		c := client.NewClient(gameServer, reader, sess, clientOpts)
		if err := c.Run(); err != nil {
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

// sanitizeUsername strips control characters and escape sequences from a username
// to prevent terminal injection attacks, then caps it to maxUsernameLength runes.
func sanitizeUsername(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	count := 0
	for _, r := range raw {
		if !unicode.IsGraphic(r) {
			continue
		}
		if count >= maxUsernameLength {
			break
		}
		b.WriteRune(r)
		count++
	}
	return strings.TrimSpace(b.String())
}
