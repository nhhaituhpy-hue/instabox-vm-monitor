package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/api"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/config"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/monitor"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
)

func main() {
	healthCheck := flag.Bool("healthcheck", false, "probe local /healthz endpoint and exit")
	flag.Parse()

	cfg := config.Load()

	if *healthCheck {
		port := strings.TrimPrefix(cfg.ListenAddr, ":")
		if strings.Contains(cfg.ListenAddr, ":") {
			parts := strings.Split(cfg.ListenAddr, ":")
			port = parts[len(parts)-1]
		}
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	masterKey, err := crypto.LoadOrCreateMasterKey(cfg.DataDir, cfg.MasterKey)
	if err != nil {
		log.Fatalf("load master key: %v", err)
	}

	st, err := store.Open(filepath.Join(cfg.DataDir, "monitor.db"), masterKey)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = st.Close() }()

	client, err := instabox.NewClient(cfg.InstaboxBaseURL, nil)
	if err != nil {
		log.Fatalf("init instabox client: %v", err)
	}

	poller := monitor.NewPoller(st, client, cfg.PollInterval, cfg.RetentionDays)
	poller.Start()
	defer poller.Stop()

	handler := api.NewRouter(st, client, poller, cfg.SessionTTL)
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("instabox-vm-monitor listening on %s (data=%s, poll=%s, retention=%dd)",
		cfg.ListenAddr, cfg.DataDir, cfg.PollInterval, cfg.RetentionDays)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}
