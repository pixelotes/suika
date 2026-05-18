package main

import (
	"flag"
	"log"

	"suika/internal/config"
	"suika/internal/server"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("suika - manga reader")
	log.Printf("loaded %d libraries", len(cfg.Libraries))

	srv := server.New(cfg)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
