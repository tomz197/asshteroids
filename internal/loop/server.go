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
}

// ClientHandle represents a client's connection to the server.
type ClientHandle struct {
	ID       int
	Player   *object.User
	Input    object.Input
	EventsCh chan ClientEvent // Events sent to client (death, etc.)
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
		kept := s.world.Objects[:0]
		for _, obj := range s.world.Objects {
			if obj != handle.Player {
				kept = append(kept, obj)
			}
		}
		s.world.Objects = kept
	}

	// Create new player at random location
	x := rand.Float64() * float64(worldWidth)
	y := rand.Float64() * float64(worldHeight)
	player := object.NewUser(x, y)
	handle.Player = player
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

	// Remove player from world
	kept := s.world.Objects[:0]
	for _, obj := range s.world.Objects {
		if obj != handle.Player {
			kept = append(kept, obj)
		}
	}
	s.world.Objects = kept
	handle.Player = nil
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
					kept := s.world.Objects[:0]
					for _, obj := range s.world.Objects {
						if obj != handle.Player {
							kept = append(kept, obj)
						}
					}
					s.world.Objects = kept
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
		// Skip players - already updated
		if s.isPlayerObject(obj) {
			kept = append(kept, obj)
			continue
		}

		remove, _ := obj.Update(ctx)
		if !remove {
			kept = append(kept, obj)
		}
	}
	s.world.Objects = kept
	s.world.FlushSpawned()

	// Check collisions
	s.checkCollisions()
}

// isPlayerObject checks if an object is a player.
func (s *Server) isPlayerObject(obj object.Object) bool {
	for _, handle := range s.clients {
		if handle.Player == obj {
			return true
		}
	}
	return false
}

// checkCollisions detects and handles collisions.
func (s *Server) checkCollisions() {
	projectiles, asteroids := collectCollidables(s.world.Objects)

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

	// Player collisions
	for _, handle := range s.clients {
		if handle.Player == nil {
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

			// Remove player
			kept := s.world.Objects[:0]
			for _, obj := range s.world.Objects {
				if obj != handle.Player {
					kept = append(kept, obj)
				}
			}
			s.world.Objects = kept
			handle.Player = nil

			// Notify client
			select {
			case handle.EventsCh <- ClientEvent{Type: EventPlayerDied}:
			default:
			}
		}
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

	// Copy objects slice (shallow copy - objects themselves are updated in place)
	objects := make([]object.Object, len(s.world.Objects))
	copy(objects, s.world.Objects)

	snapshot := &WorldSnapshot{
		Objects: objects,
		World:   s.world.World,
		Delta:   s.world.Delta,
	}

	s.snapshot.Store(snapshot)
}
