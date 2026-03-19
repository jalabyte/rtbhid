package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"rtbhid/internal/agent/config"
	"rtbhid/internal/agent/grpcclient"
	"rtbhid/internal/app"
	"rtbhid/internal/bird"
	"rtbhid/internal/httpapi"
	"rtbhid/internal/state"
	"rtbhid/internal/syncer"
)

func main() {
	mode := getEnv("RTBH_MODE", "standalone")

	if mode == "agent" {
		runAgent()
	} else {
		runStandalone()
	}
}

func runAgent() {
	portalURL := getEnv("RTBH_PORTAL_URL", "localhost:50051")
	agentName := getEnv("RTBH_AGENT_NAME", hostname())
	hostName := hostname()
	birdVersion := getEnv("RTBH_BIRD_VERSION", "2")
	birdOutput := getEnv("RTBH_BIRD_OUTPUT", "./data/rtbh_auto.conf")
	birdPeersOutput := getEnv("RTBH_BIRD_PEERS_OUTPUT", "./data/rtbh_peers_auto.conf")
	birdReload := getEnvBool("RTBH_BIRD_RELOAD", false)
	birdCmd := strings.Fields(getEnv("RTBH_BIRD_COMMAND", "birdc configure"))
	localASN := getEnvInt("RTBH_LOCAL_ASN", 65500)
	exportStd := getEnv("RTBH_EXPORT_STD_COMMUNITIES", "")
	exportLarge := getEnv("RTBH_EXPORT_LARGE_COMMUNITIES", "")

	client := grpcclient.New(grpcclient.Config{
		PortalURL:         portalURL,
		AgentName:         agentName,
		Hostname:          hostName,
		BirdVersion:       birdVersion,
		AgentVersion:      "2.0.0",
		BirdOutputPath:    birdOutput,
		BirdPeersOutput:   birdPeersOutput,
		BirdReloadEnabled: birdReload,
		BirdCommand:       birdCmd,
		LocalASN:          localASN,
		ExportStdComms:    exportStd,
		ExportLargeComms:  exportLarge,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("rtbhd agent mode\n")
	fmt.Printf("portal: %s\n", portalURL)
	fmt.Printf("agent name: %s\n", agentName)
	fmt.Printf("bird version: %s\n", birdVersion)
	fmt.Printf("bird routes output: %s\n", birdOutput)
	fmt.Printf("bird peers output: %s\n", birdPeersOutput)
	fmt.Printf("bird reload: %v\n", birdReload)
	fmt.Printf("local ASN: %d\n", localASN)

	if err := client.Run(ctx); err != nil {
		log.Fatalf("agent error: %v", err)
	}
}

func runStandalone() {
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

	fmt.Printf("rtbhd standalone on %s\n", cfg.ListenAddr)
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

func hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "agent"
	}
	return h
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
