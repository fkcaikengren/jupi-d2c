package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"d2c-manager/internal/config"
	"d2c-manager/internal/daemon"
)

func main() {
	cfg, configPath, err := config.Load()
	if err != nil {
		log.Fatalf("[d2c-manager] config error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	d, err := daemon.New(cfg, configPath)
	if err != nil {
		log.Fatalf("[d2c-manager] init error: %v", err)
	}
	if err := d.Run(ctx); err != nil {
		log.Fatalf("[d2c-manager] run error: %v", err)
	}
}
