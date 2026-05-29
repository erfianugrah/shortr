// Command shortr is the URL-shortener server binary.
//
// Subcommands:
//
//	shortr serve            run the HTTP server (default)
//	shortr migrate up       apply pending migrations
//	shortr migrate down     roll back one migration
//	shortr migrate status   print current migration state
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erfianugrah/shortr/internal/analytics"
	"github.com/erfianugrah/shortr/internal/api"
	"github.com/erfianugrah/shortr/internal/config"
	"github.com/erfianugrah/shortr/internal/identity"
	"github.com/erfianugrah/shortr/internal/obs"
	"github.com/erfianugrah/shortr/internal/shortener"
	"github.com/erfianugrah/shortr/internal/storage"
)

// version is set via -ldflags at build time.
var version = "dev"

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "serve":
		if err := serve(); err != nil {
			fmt.Fprintf(os.Stderr, "shortr serve: %v\n", err)
			os.Exit(1)
		}
	case "migrate":
		sub := "status"
		if len(os.Args) > 2 {
			sub = os.Args[2]
		}
		if err := migrate(sub); err != nil {
			fmt.Fprintf(os.Stderr, "shortr migrate %s: %v\n", sub, err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q. usage: shortr [serve|migrate|version]\n", cmd)
		os.Exit(2)
	}
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log := obs.NewLogger(cfg.Log)
	log.Info("shortr starting", "version", version, "port", cfg.HTTP.Port)

	metrics := obs.NewMetrics()

	rootCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	db, err := storage.Open(rootCtx, cfg.Storage.DBPath, log)
	if err != nil {
		return fmt.Errorf("storage open: %w", err)
	}
	defer db.Close()

	if err := storage.MigrateUp(db, log); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	// Bounded contexts wired by hand here. Each gets its repo, config slice,
	// shared logger, and shared metrics.
	linksRepo := storage.NewLinksRepo(db)
	clicksRepo := storage.NewClicksRepo(db)

	shortSvc := shortener.NewDefaultService(linksRepo, cfg.Shortener, log, metrics)
	clicksSvc := analytics.NewDefaultService(cfg.Analytics, clicksRepo, shortSvc, log, metrics)
	verifier := identity.NewBearerVerifier(cfg.Identity.AdminToken)

	// Start the click-event writer goroutine.
	clicksDone := make(chan struct{})
	go func() {
		clicksSvc.Run(rootCtx)
		close(clicksDone)
	}()

	server := api.New(api.Deps{
		HTTP:      cfg.HTTP,
		Shortener: shortSvc,
		Analytics: clicksSvc,
		Verifier:  verifier,
		Static:    staticFS(),
		Log:       log,
		Metrics:   metrics,
	})

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http listening", "addr", httpSrv.Addr)
		err := httpSrv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-rootCtx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("http: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.HTTP.ShutdownTimeout)*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown error", "err", err)
	}
	<-clicksDone
	log.Info("shortr stopped")
	return nil
}

func migrate(sub string) error {
	cfg, err := config.Load()
	if err != nil {
		// Migrations can run without ADMIN_TOKEN — synthesize one if missing.
		if cfg.Identity.AdminToken == "" {
			cfg.Identity.AdminToken = "migration-only"
		}
	}
	log := obs.NewLogger(cfg.Log)

	db, err := storage.Open(context.Background(), cfg.Storage.DBPath, log)
	if err != nil {
		return err
	}
	defer db.Close()

	switch sub {
	case "up":
		return storage.MigrateUp(db, log)
	case "down":
		return storage.MigrateDown(db, log)
	case "status":
		return storage.MigrateStatus(db, log)
	default:
		return fmt.Errorf("unknown migrate subcommand %q (use up|down|status)", sub)
	}
}
