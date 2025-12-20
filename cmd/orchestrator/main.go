package main

import (
	"context"
	"flag"
	"log"

	"github.com/joseph-gunnarsson/go-sim-local/internal/config"
	"github.com/joseph-gunnarsson/go-sim-local/internal/lb"
	"github.com/joseph-gunnarsson/go-sim-local/internal/runner"
)

func main() {
	configFile := flag.String("config", "simulation.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	runner := runner.NewRunner()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, svc := range cfg.Services {
		err := runner.StartService(ctx, svc)
		if err != nil {
			log.Fatalf("Failed to start service %s: %v", svc.Name, err)
		}
	}

	go func() {
		if err := lb.StartLB(ctx, cfg); err != nil {
			log.Printf("Load Balancer failed: %v", err)
			cancel()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		}
	}
}
