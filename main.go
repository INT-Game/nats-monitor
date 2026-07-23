package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed web/*
var webFiles embed.FS

func main() {
	configPath := flag.String("config", "config.json", "path to configuration file")
	flag.Parse()
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	c, err := newCollector(cfg.monitorURL, cfg.interval)
	if err != nil {
		log.Fatal(err)
	}
	auth, err := newAuthenticator(cfg.auth)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.run(ctx)

	assets, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           newHTTPHandler(c, auth, assets, cfg.targets),
		ReadHeaderTimeout: 5 * time.Second,
	}
	displayAddr := cfg.listenAddr
	if strings.HasPrefix(displayAddr, ":") {
		displayAddr = "127.0.0.1" + displayAddr
	}
	log.Printf("NATS Monitor listening on http://%s (target %s, refresh %s)", displayAddr, c.publicTarget, cfg.interval)
	log.Fatal(server.ListenAndServe())
}
