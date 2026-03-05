package config

import (
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type DotEnvPersister struct {
	path string
	mu   sync.Mutex
}

func NewDotEnvPersister(path string) *DotEnvPersister {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		trimmed = ".env"
	}
	return &DotEnvPersister{path: trimmed}
}

func (p *DotEnvPersister) Upsert(values map[string]string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	current := map[string]string{}
	if _, err := os.Stat(p.path); err == nil {
		existing, readErr := godotenv.Read(p.path)
		if readErr != nil {
			return readErr
		}
		current = existing
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	for key, value := range values {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		current[k] = strings.TrimSpace(value)
	}

	return godotenv.Write(current, p.path)
}
