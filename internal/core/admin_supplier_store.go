package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type AdminSupplierState struct {
	CustomCode            string    `json:"custom_code,omitempty"`
	Blocked               bool      `json:"blocked,omitempty"`
	Removed               bool      `json:"removed,omitempty"`
	AssignmentsConfigured bool      `json:"assignments_configured,omitempty"`
	AssignedItemCodes     []string  `json:"assigned_item_codes,omitempty"`
	PendingPersistCode    string    `json:"pending_persist_code,omitempty"`
	PendingPersistAt      time.Time `json:"pending_persist_at,omitempty"`
	RegenWindowStartedAt  time.Time `json:"regen_window_started_at,omitempty"`
	RegenWindowCount      int       `json:"regen_window_count,omitempty"`
	CooldownUntil         time.Time `json:"cooldown_until,omitempty"`
	UpdatedAt             time.Time `json:"updated_at,omitempty"`
}

type AdminSupplierStore struct {
	path   string
	mu     sync.Mutex
	loaded bool
	cache  map[string]AdminSupplierState
}

func NewAdminSupplierStore(path string) *AdminSupplierStore {
	return &AdminSupplierStore{path: path}
}

func (s *AdminSupplierStore) Get(ref string) (AdminSupplierState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAllLocked()
	if err != nil {
		return AdminSupplierState{}, err
	}
	return all[ref], nil
}

func (s *AdminSupplierStore) Put(ref string, state AdminSupplierState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAllLocked()
	if err != nil {
		return err
	}
	all[ref] = state
	return s.writeAllLocked(all)
}

func (s *AdminSupplierStore) Delete(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAllLocked()
	if err != nil {
		return err
	}
	delete(all, ref)
	return s.writeAllLocked(all)
}

func (s *AdminSupplierStore) List() (map[string]AdminSupplierState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.loadAllLocked()
	if err != nil {
		return nil, err
	}
	return cloneAdminSupplierStateMap(all), nil
}

func (s *AdminSupplierStore) loadAllLocked() (map[string]AdminSupplierState, error) {
	if s.loaded {
		if s.cache == nil {
			s.cache = map[string]AdminSupplierState{}
		}
		return s.cache, nil
	}
	all, err := s.readAllLocked()
	if err != nil {
		return nil, err
	}
	s.cache = all
	s.loaded = true
	return s.cache, nil
}

func (s *AdminSupplierStore) readAllLocked() (map[string]AdminSupplierState, error) {
	if _, err := os.Stat(s.path); err != nil {
		if os.IsNotExist(err) {
			return map[string]AdminSupplierState{}, nil
		}
		return nil, err
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]AdminSupplierState{}, nil
	}

	var data map[string]AdminSupplierState
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]AdminSupplierState{}
	}
	return data, nil
}

func (s *AdminSupplierStore) writeAllLocked(data map[string]AdminSupplierState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), "admin-suppliers-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	s.cache = cloneAdminSupplierStateMap(data)
	s.loaded = true
	return nil
}

func cloneAdminSupplierStateMap(input map[string]AdminSupplierState) map[string]AdminSupplierState {
	cloned := make(map[string]AdminSupplierState, len(input))
	for key, value := range input {
		value.AssignedItemCodes = append([]string(nil), value.AssignedItemCodes...)
		cloned[key] = value
	}
	return cloned
}
