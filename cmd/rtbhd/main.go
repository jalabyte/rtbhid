package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rtbhid/internal/app"
	"rtbhid/internal/bird"
	"rtbhid/internal/config"
	"rtbhid/internal/httpapi"
	"rtbhid/internal/state"
	"rtbhid/internal/syncer"
)

func main() {
	cfg := config.Load()

	store, err := state.New(cfg.StatePath)
	if err != nil {
		log.Fatalf("init state: %v", err)
	}
	exportPolicy, err := bird.ParseExportPolicy(cfg.ExportStdComms, cfg.ExportLargeComms)
	if err != nil {
		log.Fatalf("parse export communities: %v", err)
	}
	fetcher := syncer.NewFetcher(cfg.SyncUserAgent)
	birdManager := bird.NewManager(cfg.BirdOutputPath, cfg.BirdPeersOutput, cfg.BirdReloadEnabled, cfg.BirdCommand, exportPolicy)
	service := app.NewService(store, fetcher, birdManager, cfg.SchedulerTickSec, cfg.LocalASN)
	httpServer, err := httpapi.NewServer(service, httpapi.AuthConfig{
		Enabled:       cfg.AuthEnabled,
		Username:      cfg.AuthUsername,
		Password:      cfg.AuthPassword,
		SessionSecret: cfg.AuthSessionSecret,
		SessionTTL:    time.Duration(cfg.AuthSessionTTLHrs) * time.Hour,
		CookieSecure:  cfg.AuthCookieSecure,
	})
	if err != nil {
		log.Fatalf("init http auth: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	service.Start(ctx)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           httpServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Printf("rtbhd listening on %s\n", cfg.ListenAddr)
	fmt.Printf("state: %s\n", cfg.StatePath)
	fmt.Printf("bird routes output: %s\n", cfg.BirdOutputPath)
	fmt.Printf("bird peers output: %s\n", cfg.BirdPeersOutput)
	fmt.Printf("bird reload: %v (cmd: %v)\n", cfg.BirdReloadEnabled, cfg.BirdCommand)
	fmt.Printf("local asn: %d\n", cfg.LocalASN)
	fmt.Printf("export std communities: %s\n", cfg.ExportStdComms)
	fmt.Printf("export large communities: %s\n", cfg.ExportLargeComms)
	fmt.Printf("auth enabled: %v\n", cfg.AuthEnabled)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
