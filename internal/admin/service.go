package admin

import (
	"fmt"
	"strings"
	"sync"
)

type EnvPersister interface {
	Upsert(values map[string]string) error
}

type Service struct {
	mu           sync.RWMutex
	password     string
	envPersister EnvPersister
}

func NewService(password string, envPersister EnvPersister) *Service {
	return &Service{
		password:     strings.TrimSpace(password),
		envPersister: envPersister,
	}
}

func (s *Service) IsConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.password != ""
}

func (s *Service) ValidatePassword(input string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.password == "" {
		return false
	}
	return strings.TrimSpace(input) == s.password
}

func (s *Service) SetPassword(input string) error {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return fmt.Errorf("admin parol bo'sh bo'lmasligi kerak")
	}

	s.mu.Lock()
	s.password = trimmed
	s.mu.Unlock()

	if s.envPersister != nil {
		if err := s.envPersister.Upsert(map[string]string{"ADMIN_PASSWORD": trimmed}); err != nil {
			return err
		}
	}

	return nil
}
