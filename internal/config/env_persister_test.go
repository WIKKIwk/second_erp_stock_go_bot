package config

import (
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

func TestDotEnvPersisterUpsert(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	p := NewDotEnvPersister(envPath)
	if err := p.Upsert(map[string]string{
		"ERP_URL":     "https://erp.example.com",
		"ERP_API_KEY": "key-1",
	}); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	if err := p.Upsert(map[string]string{
		"ERP_API_SECRET": "secret-1",
		"ERP_API_KEY":    "key-2",
	}); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	got, err := godotenv.Read(envPath)
	if err != nil {
		t.Fatalf("failed to read persisted .env: %v", err)
	}

	if got["ERP_URL"] != "https://erp.example.com" {
		t.Fatalf("unexpected ERP_URL: %q", got["ERP_URL"])
	}
	if got["ERP_API_KEY"] != "key-2" {
		t.Fatalf("unexpected ERP_API_KEY: %q", got["ERP_API_KEY"])
	}
	if got["ERP_API_SECRET"] != "secret-1" {
		t.Fatalf("unexpected ERP_API_SECRET: %q", got["ERP_API_SECRET"])
	}
}
