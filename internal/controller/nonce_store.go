package controller

import (
	"sync"
	"time"
)

type NonceStore interface {
	Use(appID, nonce string, ttl time.Duration) bool
}

type MemoryNonceStore struct {
	mu    sync.Mutex
	items map[string]time.Time
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{items: map[string]time.Time{}}
}

func (s *MemoryNonceStore) Use(appID, nonce string, ttl time.Duration) bool {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	key := appID + ":" + nonce
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for itemKey, expiresAt := range s.items {
		if !expiresAt.After(now) {
			delete(s.items, itemKey)
		}
	}
	if expiresAt, ok := s.items[key]; ok && expiresAt.After(now) {
		return false
	}
	s.items[key] = now.Add(ttl)
	return true
}
