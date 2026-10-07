package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort         = "8080"
	defaultGoogleURL    = "https://translate-pa.googleapis.com/v1/translateHtml"
	defaultGoogleAPIKey = "AIzaSyATBXajvzQLTDHEQbcpq0Ihe0vWDHmO520"
)

type config struct {
	Port      string
	GoogleURL string
	APIKey    string
}

func loadRuntimeConfig() (config, error) {
	if err := ensureDotEnv(".env", ".env.example"); err != nil {
		return config{}, fmt.Errorf("ensure .env: %w", err)
	}
	values, err := loadDotEnv(".env")
	if err != nil {
		return config{}, fmt.Errorf("load .env: %w", err)
	}
	return loadConfig(values, os.Getenv)
}

func loadConfig(values map[string]string, getenv func(string) string) (config, error) {
	value := func(key, fallback string) string {
		raw := getenv(key)
		if raw == "" {
			raw = values[key]
		}
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			return trimmed
		}
		return fallback
	}

	cfg := config{
		Port:      value("PORT", defaultPort),
		GoogleURL: value("GOOGLE_TRANSLATE_URL", defaultGoogleURL),
		APIKey:    value("GOOGLE_TRANSLATE_API_KEY", defaultGoogleAPIKey),
	}
	port, err := strconv.ParseUint(cfg.Port, 10, 16)
	if err != nil || port == 0 {
		return config{}, errors.New("PORT must be an integer from 1 to 65535")
	}
	u, err := url.Parse(cfg.GoogleURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return config{}, errors.New("GOOGLE_TRANSLATE_URL must be an HTTP or HTTPS URL without credentials or fragment")
	}
	return cfg, nil
}
