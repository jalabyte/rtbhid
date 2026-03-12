package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr        string
	StatePath         string
	BirdOutputPath    string
	BirdPeersOutput   string
	BirdReloadEnabled bool
	BirdCommand       []string
	LocalASN          int
	SyncUserAgent     string
	SchedulerTickSec  int
	AuthEnabled       bool
	AuthUsername      string
	AuthPassword      string
	AuthSessionSecret string
	AuthSessionTTLHrs int
	AuthCookieSecure  bool
}

func Load() Config {
	cfg := Config{
		ListenAddr:        getEnv("RTBH_LISTEN_ADDR", ":8088"),
		StatePath:         getEnv("RTBH_STATE_PATH", "./data/state.json"),
		BirdOutputPath:    getEnv("RTBH_BIRD_OUTPUT", "./data/rtbh_auto.conf"),
		BirdPeersOutput:   getEnv("RTBH_BIRD_PEERS_OUTPUT", "./data/rtbh_peers_auto.conf"),
		BirdReloadEnabled: getEnvBool("RTBH_BIRD_RELOAD", false),
		BirdCommand:       strings.Fields(getEnv("RTBH_BIRD_COMMAND", "birdc configure")),
		LocalASN:          getEnvInt("RTBH_LOCAL_ASN", 65500),
		SyncUserAgent:     getEnv("RTBH_SYNC_USER_AGENT", "rtbhd/1.0"),
		SchedulerTickSec:  getEnvInt("RTBH_SCHEDULER_TICK_SECONDS", 15),
		AuthUsername:      getEnv("RTBH_AUTH_USER", ""),
		AuthPassword:      getEnv("RTBH_AUTH_PASS", ""),
		AuthSessionSecret: getEnv("RTBH_AUTH_SESSION_SECRET", ""),
		AuthSessionTTLHrs: getEnvInt("RTBH_AUTH_SESSION_TTL_HOURS", 24),
		AuthCookieSecure:  getEnvBool("RTBH_AUTH_COOKIE_SECURE", false),
	}
	if len(cfg.BirdCommand) == 0 {
		cfg.BirdCommand = []string{"birdc", "configure"}
	}
	if cfg.LocalASN <= 0 {
		cfg.LocalASN = 65500
	}
	if cfg.SchedulerTickSec < 5 {
		cfg.SchedulerTickSec = 5
	}
	if cfg.AuthSessionTTLHrs <= 0 {
		cfg.AuthSessionTTLHrs = 24
	}
	cfg.AuthEnabled = cfg.AuthUsername != "" && cfg.AuthPassword != ""
	return cfg
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
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}
