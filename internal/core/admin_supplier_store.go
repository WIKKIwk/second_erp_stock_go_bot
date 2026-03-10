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
	UpdatedAt             time.Time `json:"updated_at,omitempty"`
}

type AdminSupplierStore struct {
	path string
	mu   sync.Mutex
}

func NewAdminSupplierStore(path string) *AdminSupplierStore {
	return &AdminSupplierStore{path: path}
}

func (s *AdminSupplierStore) Get(ref string) (AdminSupplierState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAllLocked()
	if err != nil {
		return AdminSupplierState{}, err
	}
	return all[ref], nil
}

func (s *AdminSupplierStore) Put(ref string, state AdminSupplierState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAllLocked()
	if err != nil {
		return err
	}
	all[ref] = state
	return s.writeAllLocked(all)
}

func (s *AdminSupplierStore) Delete(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAllLocked()
	if err != nil {
		return err
	}
	delete(all, ref)
	return s.writeAllLocked(all)
}

func (s *AdminSupplierStore) List() (map[string]AdminSupplierState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAllLocked()
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
	return os.Rename(tmpPath, s.path)
}
