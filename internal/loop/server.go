package loop

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tomz197/asteroids/internal/object"
)

const serverTickRate = 60
const serverTickTime = time.Second / serverTickRate

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
	running      bool
	stopCh       chan struct{}

	// Double-buffered snapshot objects to avoid allocations
	snapshotBufs [2][]object.Object
	snapshotIdx  int

	// Objects marked for removal (deferred compaction)
	toRemove map[object.Object]struct{}
}

// ClientHandle represents a client's connection to the server.
type ClientHandle struct {
	ID             int
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
)

// WorldSnapshot is an immutable snapshot of the world state for rendering.
type WorldSnapshot struct {
	Objects []object.Object
	World   object.Screen
	Delta   time.Duration
}

// NewServer creates a new game server.
func NewServer() *Server {
	world := NewWorldState()
	world.World = object.Screen{
		Width:   worldWidth,
		Height:  worldHeight,
		CenterX: worldWidth / 2,
		CenterY: worldHeight / 2,
	}
	world.Screen = world.World

	s := &Server{
		world:        world,
		clients:      make(map[int]*ClientHandle),
		nextClientID: 1,
		inputChan:    make(chan ClientInput, 256),
		registerCh:   make(chan *ClientHandle, 16),
		unregisterCh: make(chan int, 16),
		stopCh:       make(chan struct{}),
		toRemove:     make(map[object.Object]struct{}),
	}

	// Create initial empty snapshot
	s.snapshot.Store(&WorldSnapshot{
		Objects: []object.Object{},
		World:   world.World,
	})

	return s
}

// Run starts the server loop. Blocks until Stop() is called.
func (s *Server) Run() {
	s.running = true
	lastTime := time.Now()

	// Add asteroid spawner
	s.world.AddObject(object.NewAsteroidSpawner(InitialAsteroidTarget))

	for s.running {
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
		if elapsed < serverTickTime {
			time.Sleep(serverTickTime - elapsed)
		}
	}
}

// Stop signals the server to stop.
func (s *Server) Stop() {
	s.running = false
	close(s.stopCh)
}

// RegisterClient registers a new client and returns its handle.
func (s *Server) RegisterClient() *ClientHandle {
	s.mu.Lock()
	id := s.nextClientID
	s.nextClientID++
	s.mu.Unlock()

	handle := &ClientHandle{
		ID:       id,
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
	x := rand.Float64() * float64(worldWidth)
	y := rand.Float64() * float64(worldHeight)
	player := object.NewUser(x, y)
	handle.Player = player
	handle.InvincibleTime = InvincibilitySeconds // Grant spawn invincibility
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
	playerSet := make(map[object.Object]struct{}, len(s.clients))
	for _, handle := range s.clients {
		if handle.Player != nil {
			playerSet[handle.Player] = struct{}{}
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
				Delta:   s.world.Delta,
				Input:   handle.Input,
				Screen:  s.world.Screen,
				Spawner: s.world,
				Objects: s.world.Objects,
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
		Delta:   s.world.Delta,
		Input:   emptyInput,
		Screen:  s.world.Screen,
		Spawner: s.world,
		Objects: s.world.Objects,
	}

	kept := s.world.Objects[:0]
	for _, obj := range s.world.Objects {
		// Skip players - already updated (O(1) lookup)
		if _, isPlayer := playerSet[obj]; isPlayer {
			kept = append(kept, obj)
			continue
		}

		remove, _ := obj.Update(ctx)
		if !remove {
			kept = append(kept, obj)
		} else {
			// Release pooled objects back to their pool
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
			if collides(p.X, p.Y, 0, a.X, a.Y, a.GetRadius()) {
				p.MarkDestroyed()
				a.MarkDestroyed()

				// Find which client owns this projectile and award score
				for _, handle := range s.clients {
					if handle.Player != nil {
						// For now, award to all playing clients
						// In future, track projectile ownership
						select {
						case handle.EventsCh <- ClientEvent{Type: EventScoreAdd, ScoreAdd: asteroidScore(a.Size)}:
						default:
						}
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

		// Check projectile hits
		for _, p := range projectiles {
			if p.IsDestroyed() {
				continue
			}
			if collides(p.X, p.Y, 0, px, py, pr) {
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
				if collides(px, py, pr, a.X, a.Y, a.GetRadius()) {
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
			if _, remove := s.toRemove[obj]; !remove {
				kept = append(kept, obj)
			}
		}
		s.world.Objects = kept
	}
}

// collides checks if two circles overlap (or point in circle if r1 == 0).
func collides(x1, y1, r1, x2, y2, r2 float64) bool {
	dx := x2 - x1
	dy := y2 - y1
	dist := dx*dx + dy*dy
	minDist := r1 + r2
	return dist < minDist*minDist
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
		Objects: buf,
		World:   s.world.World,
		Delta:   s.world.Delta,
	}

	s.snapshot.Store(snapshot)
}
