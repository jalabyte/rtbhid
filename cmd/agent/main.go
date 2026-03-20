package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
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
	birdCommand := getEnv("RTBH_BIRD_COMMAND", "birdc configure")
	birdVersion := detectBirdVersion(birdCommand, getEnv("RTBH_BIRD_VERSION", ""))
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
	fmt.Printf("export std communities: %s\n", exportStd)
	fmt.Printf("export large communities: %s\n", exportLarge)

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

// detectBirdVersion tries to detect BIRD version from birdc command.
// Falls back to envFallback if detection fails.
func detectBirdVersion(birdCommand, envFallback string) string {
	// Parse birdCommand to extract birdc binary and socket path
	// Example: "birdc -s /var/run/bird/bird.ctl configure"
	log.Printf("detectBirdVersion: parsing birdCommand='%s', envFallback='%s'", birdCommand, envFallback)
	args := strings.Fields(birdCommand)
	if len(args) == 0 {
		args = []string{"birdc"}
	}

	// Get the birdc binary (first arg)
	birdcBin := args[0]
	log.Printf("detectBirdVersion: birdc binary='%s'", birdcBin)

	// Build command with show version
	// Preserve socket path if present: -s /var/run/bird/bird.ctl
	versionArgs := []string{}
	for i := 1; i < len(args); i++ {
		if args[i] == "-s" && i+1 < len(args) {
			// Include socket path
			versionArgs = append(versionArgs, "-s", args[i+1])
			i++ // skip next arg
		} else if strings.HasPrefix(args[i], "-") {
			// Skip other flags (like configure command)
			continue
		}
	}
	versionArgs = append(versionArgs, "show", "status")
	log.Printf("detectBirdVersion: version args=%v", versionArgs)

	// Try birdc with socket path
	cmd := exec.Command(birdcBin, versionArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("detectBirdVersion: 'show status' failed: %v, output: %s", err, strings.TrimSpace(string(output)))
		// Detection failed, use env fallback or default to "2"
		if envFallback != "" {
			log.Printf("birdc detection failed, using env fallback: %s", envFallback)
			return envFallback
		}
		log.Printf("birdc detection failed, using default: 2")
		return "2"
	}
	log.Printf("birdc output: %s", strings.TrimSpace(string(output)))

	// Parse output to find version
	// Output format: "BIRD 2.0.9" or "BIRD 3.2.0" or similar
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "BIRD ") || strings.HasPrefix(line, "Bird ") {
			// Extract version number - look for version pattern after "BIRD"
			// Handle formats like: "BIRD 2.0.9" or "BIRD version 3.2.0"
			fields := strings.Fields(line)
			log.Printf("parsing line '%s', fields: %v", line, fields)
			for i, field := range fields {
				// Check if this field looks like a version number (contains dots)
				if strings.Contains(field, ".") && strings.Count(field, ".") >= 1 {
					log.Printf("found potential version field: %s", field)
					// Clean up the version string (remove any trailing characters)
					version := field
					// Extract just the version part (e.g., "2.0.9" or "3.2.0")
					for j, c := range version {
						if !((c >= '0' && c <= '9') || c == '.') {
							version = version[:j]
							break
						}
					}
					log.Printf("cleaned version: %s", version)
					// Validate it looks like a version (at least major.minor)
					parts := strings.Split(version, ".")
					if len(parts) >= 2 {
						log.Printf("returning version: %s", version)
						return version
					}
				}
				// Handle "BIRD version X.Y.Z" format
				if strings.ToLower(field) == "version" && i+1 < len(fields) {
					nextField := fields[i+1]
					if strings.Contains(nextField, ".") {
						version := nextField
						for j, c := range version {
							if !((c >= '0' && c <= '9') || c == '.') {
								version = version[:j]
								break
							}
						}
						parts := strings.Split(version, ".")
						if len(parts) >= 2 {
							return version
						}
					}
				}
			}
		}
	}

	// Could not parse version, use env fallback or default to "2"
	if envFallback != "" {
		return envFallback
	}
	return "2"
}
