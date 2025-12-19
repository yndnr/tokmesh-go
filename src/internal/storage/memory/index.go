// Package memory provides in-memory storage for TokMesh.
package memory

import (
	"sync"

	"github.com/yndnr/tokmesh-go/pkg/cmap"
)

// SessionSet is a concurrent-safe set of session IDs.
type SessionSet struct {
	mu    sync.RWMutex
	items map[string]struct{}
}

// NewSessionSet creates a new session set.
func NewSessionSet() *SessionSet {
	return &SessionSet{
		items: make(map[string]struct{}),
	}
}

// Add adds a session ID to the set.
func (s *SessionSet) Add(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = struct{}{}
}

// Remove removes a session ID from the set.
func (s *SessionSet) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
}

// Contains checks if a session ID is in the set.
func (s *SessionSet) Contains(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.items[id]
	return ok
}

// Len returns the number of items in the set.
func (s *SessionSet) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Items returns a copy of all session IDs.
func (s *SessionSet) Items() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]string, 0, len(s.items))
	for id := range s.items {
		items = append(items, id)
	}
	return items
}

// UserIndex provides secondary indexing for sessions by user.
//
// It maintains a mapping from UserID to a set of SessionIDs,
// enabling efficient lookup of all sessions for a user.
type UserIndex struct {
	index *cmap.Map[string, *SessionSet]
}

// NewUserIndex creates a new user index.
func NewUserIndex() *UserIndex {
	return &UserIndex{
		index: cmap.New[string, *SessionSet](),
	}
}

// Add adds a session to the user's session set.
func (i *UserIndex) Add(userID, sessionID string) {
	// Get or create the session set for this user
	set, _ := i.index.GetOrSet(userID, NewSessionSet())
	set.Add(sessionID)
}

// Remove removes a session from the user's session set.
func (i *UserIndex) Remove(userID, sessionID string) {
	set, ok := i.index.Get(userID)
	if !ok {
		return
	}

	set.Remove(sessionID)

	// Clean up empty sets
	if set.Len() == 0 {
		i.index.Delete(userID)
	}
}

// Get returns all session IDs for a user.
func (i *UserIndex) Get(userID string) []string {
	set, ok := i.index.Get(userID)
	if !ok {
		return nil
	}
	return set.Items()
}

// Count returns the number of sessions for a user.
func (i *UserIndex) Count(userID string) int {
	set, ok := i.index.Get(userID)
	if !ok {
		return 0
	}
	return set.Len()
}

// Clear removes all sessions for a user.
func (i *UserIndex) Clear(userID string) {
	i.index.Delete(userID)
}

// DeviceIndex provides secondary indexing for sessions by device.
//
// It maintains a mapping from DeviceID to a set of SessionIDs,
// enabling lookup of all sessions for a device.
type DeviceIndex struct {
	index *cmap.Map[string, *SessionSet]
}

// NewDeviceIndex creates a new device index.
func NewDeviceIndex() *DeviceIndex {
	return &DeviceIndex{
		index: cmap.New[string, *SessionSet](),
	}
}

// Add adds a session to the device's session set.
func (i *DeviceIndex) Add(deviceID, sessionID string) {
	if deviceID == "" {
		return
	}
	set, _ := i.index.GetOrSet(deviceID, NewSessionSet())
	set.Add(sessionID)
}

// Remove removes a session from the device's session set.
func (i *DeviceIndex) Remove(deviceID, sessionID string) {
	if deviceID == "" {
		return
	}
	set, ok := i.index.Get(deviceID)
	if !ok {
		return
	}

	set.Remove(sessionID)

	if set.Len() == 0 {
		i.index.Delete(deviceID)
	}
}

// Get returns all session IDs for a device.
func (i *DeviceIndex) Get(deviceID string) []string {
	if deviceID == "" {
		return nil
	}
	set, ok := i.index.Get(deviceID)
	if !ok {
		return nil
	}
	return set.Items()
}

// Count returns the number of sessions for a device.
func (i *DeviceIndex) Count(deviceID string) int {
	if deviceID == "" {
		return 0
	}
	set, ok := i.index.Get(deviceID)
	if !ok {
		return 0
	}
	return set.Len()
}
