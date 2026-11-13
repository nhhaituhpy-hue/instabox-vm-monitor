package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr      string
	DataDir         string
	MasterKey       string
	InstaboxBaseURL string
	PollInterval    time.Duration
	RetentionDays   int
	SessionTTL      time.Duration
}

func Load() Config {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	listenAddr := strings.TrimSpace(os.Getenv("LISTEN_ADDR"))
	if listenAddr == "" {
		listenAddr = ":" + port
	}
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" {
		dataDir = "/data"
	}
	baseURL := strings.TrimSpace(os.Getenv("INSTABOX_API_URL"))
	if baseURL == "" {
		baseURL = "https://api.instabox.sh"
	}
	pollSec := envInt("POLL_INTERVAL_SECONDS", 30)
	if pollSec < 10 {
		pollSec = 10
	}
	retDays := envInt("METRICS_RETENTION_DAYS", 7)
	if retDays < 1 {
		retDays = 1
	}
	sessHours := envInt("SESSION_TTL_HOURS", 168)
	if sessHours < 1 {
		sessHours = 24
	}
	return Config{
		ListenAddr:      listenAddr,
		DataDir:         dataDir,
		MasterKey:       os.Getenv("APP_MASTER_KEY"),
		InstaboxBaseURL: strings.TrimRight(baseURL, "/"),
		PollInterval:    time.Duration(pollSec) * time.Second,
		RetentionDays:   retDays,
		SessionTTL:      time.Duration(sessHours) * time.Hour,
	}
}

func envInt(name string, def int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}
