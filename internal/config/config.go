package config

import (
	"encoding/json"
	"errors"
	"fmt"
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
	SHMAdminBaseURL            string
	SHMAdminAuthHeaderName     string
	SHMAdminAuthHeaderValue    string
	SHMRequestTimeout          time.Duration
	TelegramBotTokensByCategory map[string]string
}

// Load reads configuration from environment variables and validates required settings.
func Load() (Config, error) {
	botTokensByCategory, err := parseStringMapEnv("TELEGRAM_BOT_TOKENS_BY_CATEGORY_JSON")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ListenAddr:                  getenv("LISTEN_ADDR", ":8080"),
		WebhookSecretHeader:         os.Getenv("WEBHOOK_SECRET_HEADER"),
		WebhookMaxClockSkew:         time.Duration(getenvInt("WEBHOOK_MAX_CLOCK_SKEW_SECONDS", 300)) * time.Second,
		SQLitePath:                  getenv("SQLITE_PATH", "/data/events.db"),
		DedupTTL:                    time.Duration(getenvInt("DEDUP_TTL_HOURS", 168)) * time.Hour,
		TelegramBotToken:            os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramParseMode:           getenv("TELEGRAM_PARSE_MODE", "HTML"),
		MessengerDryRun:             getenvBool("MESSENGER_DRY_RUN", false),
		ResolverDescriptionEnabled:  getenvBool("RESOLVER_DESCRIPTION_LOGIN_ENABLED", true),
		SHMAdminBaseURL:             strings.TrimRight(getenv("SHM_ADMIN_BASE_URL", ""), "/"),
		SHMAdminAuthHeaderName:      getenv("SHM_ADMIN_AUTH_HEADER_NAME", ""),
		SHMAdminAuthHeaderValue:     os.Getenv("SHM_ADMIN_AUTH_HEADER_VALUE"),
		SHMRequestTimeout:           time.Duration(getenvInt("SHM_REQUEST_TIMEOUT_SECONDS", 10)) * time.Second,
		TelegramBotTokensByCategory: botTokensByCategory,
	}

	if cfg.WebhookSecretHeader == "" {
		return cfg, errors.New("WEBHOOK_SECRET_HEADER is required")
	}
	if cfg.TelegramBotToken == "" && len(cfg.TelegramBotTokensByCategory) == 0 && !cfg.MessengerDryRun {
		return cfg, errors.New("TELEGRAM_BOT_TOKEN or TELEGRAM_BOT_TOKENS_BY_CATEGORY_JSON is required unless MESSENGER_DRY_RUN=true")
	}
	if len(cfg.TelegramBotTokensByCategory) > 0 && cfg.SHMAdminBaseURL == "" {
		return cfg, errors.New("SHM_ADMIN_BASE_URL is required when TELEGRAM_BOT_TOKENS_BY_CATEGORY_JSON is set")
	}
	if (cfg.SHMAdminAuthHeaderName == "") != (cfg.SHMAdminAuthHeaderValue == "") {
		return cfg, errors.New("SHM_ADMIN_AUTH_HEADER_NAME and SHM_ADMIN_AUTH_HEADER_VALUE must be set together")
	}

	return cfg, nil
}

func parseStringMapEnv(key string) (map[string]string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return map[string]string{}, nil
	}

	result := map[string]string{}
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", key, err)
	}

	for k, v := range result {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("%s must not contain empty keys or values", key)
		}
	}

	return result, nil
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
