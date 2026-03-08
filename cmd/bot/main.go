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
	"erpnext_stock_telegram/internal/suplier"
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
	supplierService := suplier.NewService(suplier.NewFileRepository("data/suppliers.fb"))
	supplierAuthService := suplier.NewAuthService(suplier.NewAuthFileRepository("data/supplier_auth.fb"))
	service := bot.NewService(
		sessions,
		credStore,
		erpClient,
		adminService,
		supplierService,
		supplierAuthService,
		cfg.SettingsPassword,
		cfg.DefaultTargetWarehouse,
		cfg.DefaultSourceWarehouse,
		cfg.DefaultUOM,
		cfg.DefaultERPURL,
		cfg.DefaultERPAPIKey,
		cfg.DefaultERPAPISecret,
		cfg.AdminkaPhone,
		cfg.AdminkaName,
		cfg.WerkaPhone,
		cfg.WerkaName,
		cfg.WerkaTelegramID,
		envPersister,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := bot.RunTelegramLoop(ctx, cfg.TelegramToken, service); err != nil {
		log.Fatalf("bot stopped with error: %v", err)
	}
}
