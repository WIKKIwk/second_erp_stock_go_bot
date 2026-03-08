package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnvReadsQuotedValues(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	content := "TELEGRAM_BOT_TOKEN=\"123:abc\"\nADMIN_PASSWORD=\"admin-pass\"\nERP_DEFAULT_UOM=\"Kg\"\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	prevDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prevDir)
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	for _, key := range []string{
		"TELEGRAM_BOT_TOKEN",
		"ADMIN_PASSWORD",
		"ERP_DEFAULT_UOM",
		"SETTINGS_PASSWORD",
		"ERP_DEFAULT_TARGET_WAREHOUSE",
		"ERP_DEFAULT_SOURCE_WAREHOUSE",
		"ERP_URL",
		"ERP_API_KEY",
		"ERP_API_SECRET",
		"ERP_TIMEOUT_SECONDS",
		"WERKA_TELEGRAM_ID",
	} {
		_ = os.Unsetenv(key)
	}

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if cfg.TelegramToken != "123:abc" {
		t.Fatalf("unexpected Telegram token: %q", cfg.TelegramToken)
	}
	if cfg.AdminPassword != "admin-pass" {
		t.Fatalf("unexpected admin password: %q", cfg.AdminPassword)
	}
	if cfg.DefaultUOM != "Kg" {
		t.Fatalf("unexpected default UOM: %q", cfg.DefaultUOM)
	}
}
