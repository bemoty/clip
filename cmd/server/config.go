package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port          string
	StoragePath   string
	AuthKey       string
	BaseURL       string
	MaxFileMB     int64
	MaxStorageMB  int64
	PasteStyle    string
	DefaultTTL    time.Duration
	SweepInterval time.Duration
}

func NewConfig() (Config, error) {
	maxFileMBStr := getEnv("MAX_FILE_MB", "100")
	maxFileMB, err := strconv.ParseInt(maxFileMBStr, 10, 64)
	if err != nil || maxFileMB <= 0 {
		return Config{}, fmt.Errorf("MAX_FILE_MB must be a positive integer, got %q", maxFileMBStr)
	}

	maxStorageMBStr := getEnv("MAX_STORAGE_MB", "0")
	maxStorageMB, err := strconv.ParseInt(maxStorageMBStr, 10, 64)
	if err != nil || maxStorageMB < 0 {
		return Config{}, fmt.Errorf("MAX_STORAGE_MB must be a non-negative integer, got %q", maxStorageMBStr)
	}

	sweepInterval, err := time.ParseDuration(getEnv("SWEEP_INTERVAL", "1h"))
	if err != nil {
		return Config{}, fmt.Errorf("SWEEP_INTERVAL must be a valid duration, got %q", getEnv("SWEEP_INTERVAL", "1h"))
	}

	var defaultTTL time.Duration
	if s := getEnv("DEFAULT_TTL", ""); s != "" {
		defaultTTL, err = parseTTL(s)
		if err != nil {
			return Config{}, fmt.Errorf("DEFAULT_TTL is invalid: %w", err)
		}
	}

	return Config{
		Port:          normalizeAddress(getEnv("PORT", ":8080")),
		StoragePath:   getEnv("STORAGE_PATH", "./data"),
		AuthKey:       getEnv("AUTH_KEY", "no-auth"),
		BaseURL:       getEnv("BASE_URL", "https://i.bemoty.dev"),
		MaxFileMB:     maxFileMB,
		MaxStorageMB:  maxStorageMB,
		PasteStyle:    getEnv("PASTE_STYLE", "dracula"),
		DefaultTTL:    defaultTTL,
		SweepInterval: sweepInterval,
	}, nil
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
