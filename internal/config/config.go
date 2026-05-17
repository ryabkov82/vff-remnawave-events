package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains runtime settings loaded from environment variables.
type Config struct {
	ListenAddr                 string
	WebhookSecretHeader        string
	WebhookMaxClockSkew        time.Duration
	SQLitePath                 string
	DedupTTL                   time.Duration
	TelegramBotToken           string
	TelegramParseMode          string
	MessengerDryRun            bool
	ResolverDescriptionEnabled bool
}

// Load reads configuration from environment variables and validates required settings.
func Load() (Config, error) {
	cfg := Config{
		ListenAddr:                 getenv("LISTEN_ADDR", ":8080"),
		WebhookSecretHeader:        os.Getenv("WEBHOOK_SECRET_HEADER"),
		WebhookMaxClockSkew:        time.Duration(getenvInt("WEBHOOK_MAX_CLOCK_SKEW_SECONDS", 300)) * time.Second,
		SQLitePath:                 getenv("SQLITE_PATH", "/data/events.db"),
		DedupTTL:                   time.Duration(getenvInt("DEDUP_TTL_HOURS", 168)) * time.Hour,
		TelegramBotToken:           os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramParseMode:          getenv("TELEGRAM_PARSE_MODE", "HTML"),
		MessengerDryRun:            getenvBool("MESSENGER_DRY_RUN", false),
		ResolverDescriptionEnabled: getenvBool("RESOLVER_DESCRIPTION_LOGIN_ENABLED", true),
	}

	if cfg.WebhookSecretHeader == "" {
		return cfg, errors.New("WEBHOOK_SECRET_HEADER is required")
	}
	if cfg.TelegramBotToken == "" && !cfg.MessengerDryRun {
		return cfg, errors.New("TELEGRAM_BOT_TOKEN is required unless MESSENGER_DRY_RUN=true")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
