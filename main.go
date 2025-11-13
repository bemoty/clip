package main

import (
	"log/slog"
	"net/http"
	"os"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(handler))

	config := LoadConfig()
	store := &DiskStore{config.StoragePath}
	server := Server{config, store}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /", server.HandleUpload)
	mux.HandleFunc("GET /{id}", server.HandleServe)
	mux.HandleFunc("GET /favicon.ico", http.NotFound)

	slog.Info("starting http server", "port", config.Port)
	if err := http.ListenAndServe(config.Port, mux); err != nil {
		slog.Error("failed to start http server", "error", err)
		os.Exit(1)
	}
}
