package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	grpcMaxMsgSize := getEnvInt("RTBH_GRPC_MAX_MSG_SIZE", 0)
	ipinfoToken := getEnv("RTBH_IPINFO_TOKEN", "")
	autoAccept := getEnvBool("RTBH_BGP_AUTO_ACCEPT", true)
	strictID := getEnvBool("RTBH_WHITELIST_STRICT_ID", true)
	idASNURL := getEnv("RTBH_ID_ASN_SOURCE_URL", "")
	idASNURLs := splitList(getEnv("RTBH_ID_ASN_SOURCE_URLS", ""))

	authCfg := httpapi.AuthConfig{
		Enabled:       os.Getenv("RTBH_AUTH_USER") != "",
		Username:      os.Getenv("RTBH_AUTH_USER"),
		Password:      os.Getenv("RTBH_AUTH_PASS"),
		SessionSecret: os.Getenv("RTBH_AUTH_SESSION_SECRET"),
		SessionTTL:    time.Duration(getEnvInt("RTBH_AUTH_SESSION_TTL_HOURS", 24)) * time.Hour,
		CookieSecure:  os.Getenv("RTBH_AUTH_COOKIE_SECURE") == "true",
	}

	store, err := state.New(dbPath)
	if err != nil {
		log.Fatalf("init state: %v", err)
	}
	defer store.Close()

	fetcher := syncer.NewFetcher("rtbhid-portal/1.0")
	svc := portal.NewService(store, fetcher, localASN, tickSec, ipinfoToken)
	svc.SetAutoAccept(autoAccept)
	svc.SetStrictIDWhitelist(strictID)
	if len(idASNURLs) > 0 {
		svc.SetIDASNSourceURLs(idASNURLs)
	} else if idASNURL != "" {
		svc.SetIDASNSourceURL(idASNURL)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	svc.Start(ctx)

	// gRPC server
	grpcServer := grpcapi.NewServer(svc, grpcAddr, grpcMaxMsgSize)
	go func() {
		if err := grpcServer.ListenAndServe(); err != nil {
			log.Fatalf("grpc server: %v", err)
		}
	}()

	// HTTP server with embedded React assets
	sub, err := fs.Sub(webAssets, "web/dist")
	if err != nil {
		log.Fatalf("sub web assets: %v", err)
	}
	httpServer, err := httpapi.NewServer(svc, sub, authCfg)
	if err != nil {
		log.Fatalf("init http server: %v", err)
	}
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
	fmt.Printf("portal gRPC max msg size: %d bytes\n", grpcMaxMsgSize)
	fmt.Printf("database: %s\n", dbPath)
	fmt.Printf("local ASN: %d\n", localASN)
	if ipinfoToken != "" {
		fmt.Printf("ipinfo.io ASN lookup: enabled\n")
	} else {
		fmt.Printf("ipinfo.io ASN lookup: disabled (set RTBH_IPINFO_TOKEN to enable)\n")
	}

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

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1" || v == "yes"
}

func splitList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
