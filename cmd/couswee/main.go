package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"couswee/internal/accounts"
	"couswee/internal/server"
	"couswee/internal/usage"
	"couswee/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("couswee", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	showVersion := flags.Bool("version", false, "print version and exit")
	flags.BoolVar(showVersion, "v", false, "print version and exit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintln(stdout, version.String())
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		return fmt.Errorf("open sqlite account store: %w", err)
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
		return err
	}
	return nil
}
