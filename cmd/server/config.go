package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port          string
	StoragePath   string
	AuthKeys      []string
	BaseURL       string
	MaxFileMB     int64
	MaxStorageMB  int64
	PasteStyle    string
	DefaultTTL    time.Duration
	SweepInterval time.Duration
}

func NewConfig() (Config, error) {
	authKeysStr := getEnv("AUTH_KEY", "no-auth")
	authKeys := strings.Split(authKeysStr, ",")
	for i, key := range authKeys {
		authKeys[i] = strings.TrimSpace(key)
	}
	if slices.Contains(authKeys, "no-auth") && authKeysStr != "no-auth" {
		return Config{}, fmt.Errorf("AUTH_KEY must not contain 'no-auth'")
	}

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
		AuthKeys:      authKeys,
		BaseURL:       getEnv("BASE_URL", "https://i.bemoty.dev"),
		MaxFileMB:     maxFileMB,
		MaxStorageMB:  maxStorageMB,
		PasteStyle:    getEnv("PASTE_STYLE", "dracula"),
		DefaultTTL:    defaultTTL,
		SweepInterval: sweepInterval,
	}, nil
}

const maxTTL = 365 * 24 * time.Hour

func parseTTL(s string) (time.Duration, error) {
	if before, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(before)
		if err != nil {
			return 0, fmt.Errorf("invalid ttl %q", s)
		}
		return validateTTL(time.Duration(n) * 24 * time.Hour)
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return validateTTL(d)
}

func validateTTL(d time.Duration) (time.Duration, error) {
	if d > maxTTL {
		return 0, errors.New("ttl too large")
	}
	if d <= 0 {
		return 0, errors.New("ttl cannot be negative or zero")
	}
	return d, nil
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
