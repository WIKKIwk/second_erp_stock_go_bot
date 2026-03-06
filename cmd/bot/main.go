package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"erpnext_stock_telegram/internal/admin"
	"erpnext_stock_telegram/internal/bot"
	"erpnext_stock_telegram/internal/config"
	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/store"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
	erpClient := erpnext.NewClient(httpClient)
	sessions := bot.NewSessionManager()
	credStore := store.NewMemoryCredentialStore()
	envPersister := config.NewDotEnvPersister(".env")
	adminService := admin.NewService(cfg.AdminPassword, envPersister)
	service := bot.NewService(
		sessions,
		credStore,
		erpClient,
		adminService,
		cfg.SettingsPassword,
		cfg.DefaultTargetWarehouse,
		cfg.DefaultSourceWarehouse,
		cfg.DefaultUOM,
		cfg.DefaultERPURL,
		cfg.DefaultERPAPIKey,
		cfg.DefaultERPAPISecret,
		envPersister,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := bot.RunTelegramLoop(ctx, cfg.TelegramToken, service); err != nil {
		log.Fatalf("bot stopped with error: %v", err)
	}
}
