package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(handler))

	config, err := NewConfig()
	if err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := NewDiskStore(ctx, config)
	server := Server{config, store}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /", server.HandleUpload)
	mux.HandleFunc("GET /{id}", server.HandleServe)
	mux.HandleFunc("GET /", http.NotFound)
	mux.HandleFunc("GET /favicon.ico", http.NotFound)

	slog.Info("starting http server", "port", config.Port)
	if err := http.ListenAndServe(config.Port, mux); err != nil {
		slog.Error("failed to start http server", "error", err)
		os.Exit(1)
	}
}
