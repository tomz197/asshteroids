package server

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tomz197/asteroids/internal/loop/config"
	"github.com/tomz197/asteroids/internal/object"
	"github.com/tomz197/asteroids/internal/physics"
)

// GameServer is the interface clients use to communicate with the game server.
// Decouples the Client from the concrete Server implementation, enabling
// testing and potential network-based server implementations.
type GameServer interface {
	RegisterClient(username string) *ClientHandle
	UnregisterClient(clientID int)
	SendInput(clientID int, input object.Input)
	GetSnapshot() *WorldSnapshot
	GetClientPlayer(clientID int) *object.User
	SpawnPlayer(clientID int)
	RemovePlayer(clientID int)
}

// Server manages the shared world state and processes inputs from all clients.
type Server struct {
	world        *WorldState
	snapshot     atomic.Pointer[WorldSnapshot]
	clients      map[int]*ClientHandle
	nextClientID int
	inputChan    chan ClientInput
	registerCh   chan *ClientHandle
	unregisterCh chan int
	mu           sync.RWMutex

	// Double-buffered snapshot objects to avoid allocations
	snapshotBufs [2][]object.Object
	snapshotIdx  int

	// Objects marked for removal (deferred compaction)
	toRemove map[object.Object]struct{}

	// Reusable player set to avoid per-frame allocation
	playerSet map[object.Object]struct{}
}

// Compile-time check that Server implements GameServer.
var _ GameServer = (*Server)(nil)

// ClientHandle represents a client's connection to the server.
type ClientHandle struct {
	ID             int
	Username       string // Display name for this client
	Player         *object.User
	Input          object.Input
	EventsCh       chan ClientEvent // Events sent to client (death, etc.)
	InvincibleTime float64          // Remaining invincibility time in seconds
}

// ClientInput represents input from a specific client.
type ClientInput struct {
	ClientID int
	Input    object.Input
}

// ClientEvent represents an event sent from server to client.
type ClientEvent struct {
	Type     ClientEventType
	KilledBy string // For death events
	ScoreAdd int    // For score events
}

// ClientEventType identifies the type of client event.
type ClientEventType int

const (
	EventPlayerDied ClientEventType = iota
	EventScoreAdd
	EventServerShutdown
)

// NewServer creates a new game server.
func NewServer() *Server {
	world := NewWorldState()
	world.World = object.Screen{
		Width:   config.WorldWidth,
		Height:  config.WorldHeight,
		CenterX: config.WorldWidth / 2,
		CenterY: config.WorldHeight / 2,
	}
	world.Screen = world.World

	s := &Server{
		world:        world,
		clients:      make(map[int]*ClientHandle),
		nextClientID: 1,
		inputChan:    make(chan ClientInput, 256),
		registerCh:   make(chan *ClientHandle, 16),
		unregisterCh: make(chan int, 16),
		toRemove:     make(map[object.Object]struct{}),
		playerSet:    make(map[object.Object]struct{}),
	}

	// Create initial empty snapshot
	s.snapshot.Store(&WorldSnapshot{
		Objects: []object.Object{},
		World:   world.World,
	})

	return s
}

// Run starts the server loop. Blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) {
	lastTime := time.Now()

	// Add asteroid spawner
	s.world.AddObject(object.NewAsteroidSpawner(config.InitialAsteroidTarget))

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		frameStart := time.Now()
		s.world.Delta = frameStart.Sub(lastTime)
		lastTime = frameStart

		// Process registrations/unregistrations
		s.processRegistrations()

		// Collect all pending inputs
		s.collectInputs()

		// Update world state
		s.updateWorld()

		// Create new snapshot for clients
		s.createSnapshot()

		// Frame timing
		elapsed := time.Since(frameStart)
		if elapsed < config.ServerTickTime {
			time.Sleep(config.ServerTickTime - elapsed)
		}
	}
}

// Shutdown gracefully shuts down the server by notifying all connected clients
// and waiting for them to disconnect (up to the given timeout).
// The caller should cancel the server context after Shutdown returns.
func (s *Server) Shutdown(timeout time.Duration) {
	// Notify all connected clients about the shutdown
	s.mu.RLock()
	for _, handle := range s.clients {
		select {
		case handle.EventsCh <- ClientEvent{Type: EventServerShutdown}:
		default:
		}
	}
	s.mu.RUnlock()

	// Wait for all clients to disconnect, or timeout
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return
		case <-ticker.C:
			s.mu.RLock()
			remaining := len(s.clients)
			s.mu.RUnlock()
			if remaining == 0 {
				return
			}
		}
	}
}

// RegisterClient registers a new client with the given username and returns its handle.
func (s *Server) RegisterClient(username string) *ClientHandle {
	s.mu.Lock()
	id := s.nextClientID
	s.nextClientID++
	s.mu.Unlock()

	handle := &ClientHandle{
		ID:       id,
		Username: username,
		EventsCh: make(chan ClientEvent, 16),
	}

	s.registerCh <- handle
	return handle
}

// UnregisterClient removes a client from the server.
func (s *Server) UnregisterClient(clientID int) {
	s.unregisterCh <- clientID
}

// SendInput sends input from a client to the server.
func (s *Server) SendInput(clientID int, input object.Input) {
	select {
	case s.inputChan <- ClientInput{ClientID: clientID, Input: input}:
	default:
		// Input channel full, drop input
	}
}

// GetSnapshot returns the current world snapshot.
func (s *Server) GetSnapshot() *WorldSnapshot {
	return s.snapshot.Load()
}

// GetClientPlayer returns the player object for a client (thread-safe).
func (s *Server) GetClientPlayer(clientID int) *object.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if handle, ok := s.clients[clientID]; ok {
		return handle.Player
	}
	return nil
}

// SpawnPlayer spawns a player for the given client.
func (s *Server) SpawnPlayer(clientID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	handle, ok := s.clients[clientID]
	if !ok {
		return
	}

	// Remove existing player if any
	if handle.Player != nil {
		s.removeObjectLocked(handle.Player)
	}

	// Create new player at random location
	x := rand.Float64() * float64(config.WorldWidth)
	y := rand.Float64() * float64(config.WorldHeight)
	player := object.NewUser(x, y)
	player.OwnerID = clientID
	player.Username = handle.Username
	handle.Player = player
	handle.InvincibleTime = config.InvincibilitySeconds // Grant spawn invincibility
	s.world.AddObject(player)
}

// RemovePlayer removes the player for a client.
func (s *Server) RemovePlayer(clientID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	handle, ok := s.clients[clientID]
	if !ok || handle.Player == nil {
		return
	}

	s.removeObjectLocked(handle.Player)
	handle.Player = nil
}

// removeObjectLocked removes a single object from the world. Must be called with lock held.
func (s *Server) removeObjectLocked(target object.Object) {
	s.world.RemoveObject(target)
	kept := s.world.Objects[:0]
	for _, obj := range s.world.Objects {
		if obj != target {
			kept = append(kept, obj)
		}
	}
	s.world.Objects = kept
}

// processRegistrations handles pending client registrations/unregistrations.
func (s *Server) processRegistrations() {
	for {
		select {
		case handle := <-s.registerCh:
			s.mu.Lock()
			s.clients[handle.ID] = handle
			s.mu.Unlock()
		case clientID := <-s.unregisterCh:
			s.mu.Lock()
			if handle, ok := s.clients[clientID]; ok {
				// Remove player from world
				if handle.Player != nil {
					s.removeObjectLocked(handle.Player)
				}
				close(handle.EventsCh)
				delete(s.clients, clientID)
			}
			s.mu.Unlock()
		default:
			return
		}
	}
}

// collectInputs gathers all pending inputs from clients.
func (s *Server) collectInputs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		select {
		case ci := <-s.inputChan:
			if handle, ok := s.clients[ci.ClientID]; ok {
				handle.Input = ci.Input
			}
		default:
			return
		}
	}
}

// updateWorld updates the world state based on collected inputs.
func (s *Server) updateWorld() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Decrement invincibility timers and build player set for O(1) lookup
	dt := s.world.Delta.Seconds()

	// Reuse player set to avoid per-frame allocation
	clear(s.playerSet)
	for _, handle := range s.clients {
		if handle.Player != nil {
			s.playerSet[handle.Player] = struct{}{}
		}
		if handle.InvincibleTime > 0 {
			handle.InvincibleTime -= dt
			if handle.InvincibleTime < 0 {
				handle.InvincibleTime = 0
			}
		}
	}

	// Update each player with their input
	for _, handle := range s.clients {
		if handle.Player != nil {
			ctx := object.UpdateContext{
				Delta:         s.world.Delta,
				Input:         handle.Input,
				Screen:        s.world.Screen,
				Spawner:       s.world,
				Objects:       s.world.Objects,
				AsteroidCount: s.world.AsteroidCount,
			}
			remove, _ := handle.Player.Update(ctx)
			if remove {
				handle.Player = nil
			}
		}
	}

	// Update non-player objects with empty input
	emptyInput := object.Input{}
	ctx := object.UpdateContext{
		Delta:         s.world.Delta,
		Input:         emptyInput,
		Screen:        s.world.Screen,
		Spawner:       s.world,
		Objects:       s.world.Objects,
		AsteroidCount: s.world.AsteroidCount,
	}

	kept := s.world.Objects[:0]
	for _, obj := range s.world.Objects {
		// Skip players - already updated (O(1) lookup)
		if _, isPlayer := s.playerSet[obj]; isPlayer {
			kept = append(kept, obj)
			continue
		}

		remove, _ := obj.Update(ctx)
		if !remove {
			kept = append(kept, obj)
		} else {
			// Decrement tracked counts and release pooled objects
			s.world.RemoveObject(obj)
			object.ReleaseObject(obj)
		}
	}
	s.world.Objects = kept
	s.world.FlushSpawned()

	// Check collisions
	s.checkCollisions()
}

// checkCollisions detects and handles collisions.
func (s *Server) checkCollisions() {
	// Use cached slices from world state
	collectCollidables(s.world.Objects, &s.world.projectileCache, &s.world.asteroidCache)
	projectiles := s.world.projectileCache
	asteroids := s.world.asteroidCache

	// Clear removal set for this frame
	clear(s.toRemove)

	// Projectile-asteroid collisions
	for _, p := range projectiles {
		if p.IsDestroyed() {
			continue
		}
		for _, a := range asteroids {
			if a.IsDestroyed() || a.IsProtected() {
				continue
			}
			if physics.PointInCircle(p.X, p.Y, a.X, a.Y, a.GetRadius()) {
				p.MarkDestroyed()
				a.MarkDestroyed()

				// Award score to the client that owns this projectile
				if handle, ok := s.clients[p.OwnerID]; ok {
					select {
					case handle.EventsCh <- ClientEvent{Type: EventScoreAdd, ScoreAdd: asteroidScore(a.Size)}:
					default:
					}
				}
			}
		}
	}

	// Projectile-projectile collisions
	checkProjectileProjectileCollisions(projectiles)

	// Asteroid-asteroid collisions
	checkAsteroidAsteroidCollisions(asteroids)

	// Player collisions (skip invincible players)
	for _, handle := range s.clients {
		if handle.Player == nil || handle.InvincibleTime > 0 {
			continue
		}
		px, py := handle.Player.GetPosition()
		pr := handle.Player.GetRadius()

		hit := false

		// Check projectile hits (skip own projectiles)
		for _, p := range projectiles {
			if p.IsDestroyed() || p.OwnerID == handle.ID {
				continue
			}
			if physics.PointInCircle(p.X, p.Y, px, py, pr) {
				p.MarkDestroyed()
				hit = true
				break
			}
		}

		// Check asteroid collisions
		if !hit {
			for _, a := range asteroids {
				if a.IsDestroyed() || a.IsProtected() {
					continue
				}
				if physics.CirclesOverlap(px, py, pr, a.X, a.Y, a.GetRadius()) {
					hit = true
					break
				}
			}
		}

		if hit {
			// Spawn death explosion
			x, y := handle.Player.GetPosition()
			object.SpawnExplosion(x, y, 20, 25.0, 1.0, s.world)

			// Mark player for removal (deferred compaction)
			s.toRemove[handle.Player] = struct{}{}
			handle.Player = nil

			// Notify client
			select {
			case handle.EventsCh <- ClientEvent{Type: EventPlayerDied}:
			default:
			}
		}
	}

	// Perform deferred compaction if needed
	if len(s.toRemove) > 0 {
		kept := s.world.Objects[:0]
		for _, obj := range s.world.Objects {
			if _, remove := s.toRemove[obj]; remove {
				s.world.RemoveObject(obj)
			} else {
				kept = append(kept, obj)
			}
		}
		s.world.Objects = kept
	}
}

// createSnapshot creates an immutable snapshot of the world state.
func (s *Server) createSnapshot() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use double-buffered slice to avoid allocations
	idx := s.snapshotIdx
	s.snapshotIdx = 1 - s.snapshotIdx // Toggle for next frame

	// Grow buffer if needed, otherwise reuse
	buf := s.snapshotBufs[idx]
	if cap(buf) < len(s.world.Objects) {
		buf = make([]object.Object, len(s.world.Objects))
		s.snapshotBufs[idx] = buf
	}
	buf = buf[:len(s.world.Objects)]
	copy(buf, s.world.Objects)

	snapshot := &WorldSnapshot{
		Objects:     buf,
		UserObjects: object.FilterUsers(buf),
		Players:     len(s.clients),
		World:       s.world.World,
		Delta:       s.world.Delta,
	}

	s.snapshot.Store(snapshot)
}
