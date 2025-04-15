package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.rikki.moe/v2stat"
)

func runDaemon(v2s *v2stat.V2Stat) {
	if err := v2s.InitDB(); err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(time.Duration(*flagInterval) * time.Second)
	defer ticker.Stop()

	for {
		logger.Info("Recording stats...")
		if err := v2s.RecordNow(ctx); err != nil {
			logger.Errorf("Failed to record stats: %v", err)
		}

		select {
		case <-ticker.C:
			continue
		case <-sigCh:
			logger.Info("Received shutdown signal, exiting.")
			return
		case <-ctx.Done():
			logger.Info("Context canceled, exiting.")
			return
		}
	}
}
