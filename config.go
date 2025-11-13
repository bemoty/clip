package main

import (
	"os"
	"strings"
)

type Config struct {
	Port        string
	StoragePath string
	AuthKey     string
	BaseURL     string
}

func LoadConfig() Config {
	return Config{
		Port:        normalizeAddress(getEnv("PORT", ":8080")),
		StoragePath: getEnv("STORAGE_PATH", "./data"),
		AuthKey:     getEnv("AUTH_KEY", "no-auth"),
		BaseURL:     getEnv("BASE_URL", "https://i.bemoty.dev"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func normalizeAddress(addr string) string {
	if !strings.HasPrefix(addr, ":") {
		return ":" + addr
	}
	return addr
}
