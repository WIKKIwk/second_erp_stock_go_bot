package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"erpnext_stock_telegram/internal/config"
	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/mobileapi"
)

func main() {
	addr := strings.TrimSpace(os.Getenv("MOBILE_API_ADDR"))
	if addr == "" {
		addr = ":8081"
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	erpClient := erpnext.NewClient(&http.Client{Timeout: cfg.RequestTimeout})
	auth := mobileapi.NewERPAuthenticator(
		erpClient,
		cfg.DefaultERPURL,
		cfg.DefaultERPAPIKey,
		cfg.DefaultERPAPISecret,
		cfg.DefaultTargetWarehouse,
		os.Getenv("MOBILE_DEV_SUPPLIER_PREFIX"),
		os.Getenv("MOBILE_DEV_WERKA_PREFIX"),
		os.Getenv("MOBILE_DEV_WERKA_CODE"),
		cfg.WerkaPhone,
		os.Getenv("MOBILE_DEV_WERKA_NAME"),
	)

	server := mobileapi.NewServer(auth)
	log.Printf("mobile API listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("mobile API stopped: %v", err)
	}
}
