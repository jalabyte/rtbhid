package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rtbhid/internal/portal"
	"rtbhid/internal/portal/grpcapi"
	"rtbhid/internal/portal/httpapi"
	"rtbhid/internal/portal/state"
	"rtbhid/internal/syncer"
)

//go:embed all:web/dist
var webAssets embed.FS

func main() {
	listenAddr := getEnv("RTBH_LISTEN_ADDR", ":8088")
	grpcAddr := getEnv("RTBH_GRPC_ADDR", ":50051")
	dbPath := getEnv("RTBH_DB_PATH", "./data/portal.db")
	localASN := getEnvInt("RTBH_LOCAL_ASN", 65500)
	tickSec := getEnvInt("RTBH_SCHEDULER_TICK_SECONDS", 15)

	store, err := state.New(dbPath)
	if err != nil {
		log.Fatalf("init state: %v", err)
	}
	defer store.Close()

	fetcher := syncer.NewFetcher("rtbhid-portal/1.0")
	svc := portal.NewService(store, fetcher, localASN, tickSec)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	svc.Start(ctx)

	// gRPC server
	grpcServer := grpcapi.NewServer(svc, grpcAddr)
	go func() {
		if err := grpcServer.ListenAndServe(); err != nil {
			log.Fatalf("grpc server: %v", err)
		}
	}()

	// HTTP server with embedded React assets
	httpServer := httpapi.NewServer(svc, webAssets)
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           httpServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Printf("portal HTTP listening on %s\n", listenAddr)
	fmt.Printf("portal gRPC listening on %s\n", grpcAddr)
	fmt.Printf("database: %s\n", dbPath)
	fmt.Printf("local ASN: %d\n", localASN)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	fmt.Sscanf(v, "%d", &n)
	if n <= 0 {
		return fallback
	}
	return n
}
