package main

import (
	"log"
	"net/http"

	"github.com/ryabkov82/vff-remnawave-events/internal/config"
	"github.com/ryabkov82/vff-remnawave-events/internal/dedup"
	"github.com/ryabkov82/vff-remnawave-events/internal/httpserver"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	store, err := dedup.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("dedup store open error: %v", err)
	}
	defer store.Close()

	server := httpserver.NewDefault(cfg, store)

	log.Printf("starting vff-remnawave-events on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, server.Routes()); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}
