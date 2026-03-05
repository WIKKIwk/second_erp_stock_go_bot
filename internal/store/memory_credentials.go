package store

import "sync"

type MemoryCredentialStore struct {
	mu    sync.RWMutex
	items map[int64]Credentials
}

func NewMemoryCredentialStore() *MemoryCredentialStore {
	return &MemoryCredentialStore{
		items: make(map[int64]Credentials),
	}
}

func (s *MemoryCredentialStore) Save(chatID int64, creds Credentials) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[chatID] = creds
}

func (s *MemoryCredentialStore) Get(chatID int64) (Credentials, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	creds, ok := s.items[chatID]
	return creds, ok
}
