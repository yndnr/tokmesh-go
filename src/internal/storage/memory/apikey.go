// Package memory provides in-memory storage implementations.
package memory

import (
	"context"
	"sync"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// APIKeyStore provides in-memory storage for API keys.
//
// @design DS-0103
type APIKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*domain.APIKey
}

// NewAPIKeyStore creates a new API key store.
func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{
		keys: make(map[string]*domain.APIKey),
	}
}

// Get retrieves an API key by ID.
func (s *APIKeyStore) Get(_ context.Context, keyID string) (*domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.keys[keyID]
	if !ok {
		return nil, domain.ErrAPIKeyNotFound
	}

	return key.Clone(), nil
}

// Create creates a new API key.
func (s *APIKeyStore) Create(_ context.Context, key *domain.APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.keys[key.KeyID]; exists {
		return domain.ErrAPIKeyConflict
	}

	s.keys[key.KeyID] = key.Clone()
	return nil
}

// Update updates an existing API key.
func (s *APIKeyStore) Update(_ context.Context, key *domain.APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.keys[key.KeyID]; !exists {
		return domain.ErrAPIKeyNotFound
	}

	s.keys[key.KeyID] = key.Clone()
	return nil
}

// Delete deletes an API key by ID.
func (s *APIKeyStore) Delete(_ context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.keys[keyID]; !exists {
		return domain.ErrAPIKeyNotFound
	}

	delete(s.keys, keyID)
	return nil
}

// List retrieves all API keys.
func (s *APIKeyStore) List(_ context.Context) ([]*domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]*domain.APIKey, 0, len(s.keys))
	for _, key := range s.keys {
		keys = append(keys, key.Clone())
	}

	return keys, nil
}
