package main

import (
	"flag"
	"log"

	"github.com/Zereker/memory/internal/server"
)

var (
	configFile = flag.String("config", "configs/config.toml", "Path to config file")
)

func init() {
	flag.Parse()
}

func main() {
	conf, err := server.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	srv, err := server.NewServer(conf)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	defer func() { _ = srv.Shutdown() }()

	if err = srv.Start(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
