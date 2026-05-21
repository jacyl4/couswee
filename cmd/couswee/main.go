package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"couswee/internal/accounts"
	"couswee/internal/server"
	"couswee/internal/usage"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("resolve home directory: %v", err)
	}

	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		log.Fatalf("open sqlite account store: %v", err)
	}
	defer store.Close()

	service := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	service.UseCodexLoginRunner()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	usageCfg := usage.ConfigFromEnv()
	usageCfg.ActiveAuthPath = service.CurrentAuthPath()
	usageService := usage.NewService(usageCfg, usage.BuildCollector(usageCfg), service.Accounts)
	usageService.SetAccountSink(service.ReplaceUsage)
	usageService.RefreshAll(ctx)
	usageService.Start(ctx)

	addr := os.Getenv("COUSWEE_ADDR")
	if addr == "" {
		addr = "127.0.0.1:2199"
	}
	staticDir := os.Getenv("COUSWEE_STATIC_DIR")
	if staticDir == "" {
		staticDir = "web/dist"
	}

	app := server.New(service, server.Config{StaticDir: staticDir, Usage: usageService})
	log.Printf("couswee listening on http://%s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatal(err)
	}
}
