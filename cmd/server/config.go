package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	StoragePath string
	AuthKey     string
	BaseURL     string
	MaxFileMB   int64
	PasteStyle  string
}

func LoadConfig() (Config, error) {
	maxFileMBStr := getEnv("MAX_FILE_MB", "100")
	maxFileMB, err := strconv.ParseInt(maxFileMBStr, 10, 64)
	if err != nil || maxFileMB <= 0 {
		return Config{}, fmt.Errorf("MAX_FILE_MB must be a positive integer, got %q", maxFileMBStr)
	}
	return Config{
		Port:        normalizeAddress(getEnv("PORT", ":8080")),
		StoragePath: getEnv("STORAGE_PATH", "./data"),
		AuthKey:     getEnv("AUTH_KEY", "no-auth"),
		BaseURL:     getEnv("BASE_URL", "https://i.bemoty.dev"),
		MaxFileMB:   maxFileMB,
		PasteStyle:  getEnv("PASTE_STYLE", "dracula"),
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
