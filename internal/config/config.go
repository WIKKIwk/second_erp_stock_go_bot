package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken          string
	RequestTimeout         time.Duration
	SettingsPassword       string
	DefaultTargetWarehouse string
	DefaultSourceWarehouse string
	DefaultUOM             string
}

func LoadFromEnv() (Config, error) {
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	timeout := 15 * time.Second
	if raw := os.Getenv("ERP_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Config{}, fmt.Errorf("invalid ERP_TIMEOUT_SECONDS: %q", raw)
		}
		timeout = time.Duration(seconds) * time.Second
	}

	return Config{
		TelegramToken:          token,
		RequestTimeout:         timeout,
		SettingsPassword:       os.Getenv("SETTINGS_PASSWORD"),
		DefaultTargetWarehouse: os.Getenv("ERP_DEFAULT_TARGET_WAREHOUSE"),
		DefaultSourceWarehouse: os.Getenv("ERP_DEFAULT_SOURCE_WAREHOUSE"),
		DefaultUOM:             os.Getenv("ERP_DEFAULT_UOM"),
	}, nil
}
