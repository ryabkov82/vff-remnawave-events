package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Config struct {
	ListenAddr                  string
	WebhookSecretHeader         string
	WebhookMaxClockSkew         time.Duration
	SQLitePath                  string
	DedupTTL                    time.Duration
	TelegramBotToken            string
	TelegramParseMode           string
	ResolverDescriptionEnabled  bool
}

type RemnawaveEvent struct {
	Scope     string    `json:"scope"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      EventData `json:"data"`
}

type EventData struct {
	Node   NodeData   `json:"node"`
	User   UserData   `json:"user"`
	Report ReportData `json:"report"`
}

type NodeData struct {
	UUID    string `json:"uuid"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

type UserData struct {
	UUID            string          `json:"uuid"`
	ID              json.RawMessage `json:"id"`
	ShortUUID       string          `json:"shortUuid"`
	Username        string          `json:"username"`
	Status          string          `json:"status"`
	TelegramID      json.RawMessage `json:"telegramId"`
	Email           string          `json:"email"`
	Description     string          `json:"description"`
	SubscriptionURL string          `json:"subscriptionUrl"`
}

type ReportData struct {
	ActionReport ActionReport `json:"actionReport"`
	XrayReport   XrayReport   `json:"xrayReport"`
}

type ActionReport struct {
	Blocked       bool   `json:"blocked"`
	IP            string `json:"ip"`
	BlockDuration int    `json:"blockDuration"`
	WillUnblockAt string `json:"willUnblockAt"`
	UserID        string `json:"userId"`
	ProcessedAt   string `json:"processedAt"`
}

type XrayReport struct {
	Email       string `json:"email"`
	Protocol    string `json:"protocol"`
	Network     string `json:"network"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	InboundTag  string `json:"inboundTag"`
	OutboundTag string `json:"outboundTag"`
}

type Server struct {
	cfg       Config
	db        *sql.DB
	loginExpr *regexp.Regexp
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := sql.Open("sqlite3", cfg.SQLitePath)
	if err != nil {
		log.Fatalf("sqlite open error: %v", err)
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		log.Fatalf("sqlite init error: %v", err)
	}

	s := &Server{
		cfg:       cfg,
		db:        db,
		loginExpr: regexp.MustCompile(`login=@?([0-9]{5,20})`),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /remnawave", s.remnawave)

	log.Printf("starting vff-remnawave-events on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

func loadConfig() (Config, error) {
	cfg := Config{
		ListenAddr:                 getenv("LISTEN_ADDR", ":8080"),
		WebhookSecretHeader:        os.Getenv("WEBHOOK_SECRET_HEADER"),
		WebhookMaxClockSkew:        time.Duration(getenvInt("WEBHOOK_MAX_CLOCK_SKEW_SECONDS", 300)) * time.Second,
		SQLitePath:                 getenv("SQLITE_PATH", "/data/events.db"),
		DedupTTL:                   time.Duration(getenvInt("DEDUP_TTL_HOURS", 168)) * time.Hour,
		TelegramBotToken:           os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramParseMode:          getenv("TELEGRAM_PARSE_MODE", "HTML"),
		ResolverDescriptionEnabled: getenvBool("RESOLVER_DESCRIPTION_LOGIN_ENABLED", true),
	}
	if cfg.WebhookSecretHeader == "" {
		return cfg, errors.New("WEBHOOK_SECRET_HEADER is required")
	}
	if cfg.TelegramBotToken == "" {
		return cfg, errors.New("TELEGRAM_BOT_TOKEN is required")
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

func initDB(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS processed_events (
    event_key TEXT PRIMARY KEY,
    event_ts TEXT NOT NULL,
    remnawave_user_id TEXT,
    remnawave_username TEXT,
    node_name TEXT,
    source_ip TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_processed_events_created_at ON processed_events(created_at);
`)
	return err
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) remnawave(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if err := s.verifyRequest(r, body); err != nil {
		log.Printf("invalid webhook request: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var event RemnawaveEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("invalid json: %v", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if !isTorrentBlockEvent(event) {
		log.Printf("ignored event scope=%s event=%s", event.Scope, event.Event)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	if err := s.deleteOldEvents(); err != nil {
		log.Printf("dedup ttl cleanup failed: %v", err)
	}

	eventKey := buildEventKey(event)
	inserted, err := s.insertEvent(eventKey, event)
	if err != nil {
		log.Printf("dedup insert failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !inserted {
		log.Printf("duplicate torrent blocker event key=%s user=%s node=%s", eventKey, rawToString(event.Data.User.ID), event.Data.Node.Name)
		writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
		return
	}

	chatID, ok := s.resolveTelegramChatID(event.Data.User)
	if !ok {
		log.Printf("telegram chat id not found user_id=%s username=%s description=%q", rawToString(event.Data.User.ID), event.Data.User.Username, event.Data.User.Description)
		writeJSON(w, http.StatusOK, map[string]string{"status": "no_recipient"})
		return
	}

	if err := s.sendTelegram(chatID, buildUserMessage(event)); err != nil {
		log.Printf("telegram send failed chat_id=%s user_id=%s error=%v", chatID, rawToString(event.Data.User.ID), err)
		writeJSON(w, http.StatusOK, map[string]string{"status": "telegram_failed"})
		return
	}

	log.Printf("telegram notification sent chat_id=%s user_id=%s username=%s node=%s", chatID, rawToString(event.Data.User.ID), event.Data.User.Username, event.Data.Node.Name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Server) verifyRequest(r *http.Request, body []byte) error {
	signature := strings.TrimSpace(r.Header.Get("X-Remnawave-Signature"))
	if signature == "" {
		return errors.New("missing X-Remnawave-Signature")
	}

	mac := hmac.New(sha256.New, []byte(s.cfg.WebhookSecretHeader))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return errors.New("signature mismatch")
	}

	timestampHeader := strings.TrimSpace(r.Header.Get("X-Remnawave-Timestamp"))
	if timestampHeader == "" {
		return errors.New("missing X-Remnawave-Timestamp")
	}
	parsed, err := time.Parse(time.RFC3339Nano, timestampHeader)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	age := time.Since(parsed)
	if age < -s.cfg.WebhookMaxClockSkew || age > s.cfg.WebhookMaxClockSkew {
		return fmt.Errorf("timestamp outside allowed skew: %s", age)
	}

	return nil
}

func isTorrentBlockEvent(event RemnawaveEvent) bool {
	return event.Scope == "torrent_blocker" &&
		event.Event == "torrent_blocker.report" &&
		event.Data.Report.ActionReport.Blocked
}

func buildEventKey(event RemnawaveEvent) string {
	parts := []string{
		event.Scope,
		event.Event,
		rawToString(event.Data.User.ID),
		event.Data.User.UUID,
		event.Data.Node.UUID,
		event.Data.Report.ActionReport.ProcessedAt,
		event.Data.Report.ActionReport.IP,
		event.Data.Report.XrayReport.Source,
		event.Data.Report.XrayReport.Destination,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func (s *Server) insertEvent(eventKey string, event RemnawaveEvent) (bool, error) {
	_, err := s.db.Exec(`
INSERT INTO processed_events (event_key, event_ts, remnawave_user_id, remnawave_username, node_name, source_ip)
VALUES (?, ?, ?, ?, ?, ?)
`, eventKey, event.Timestamp.Format(time.RFC3339Nano), rawToString(event.Data.User.ID), event.Data.User.Username, event.Data.Node.Name, event.Data.Report.ActionReport.IP)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Server) deleteOldEvents() error {
	cutoff := time.Now().Add(-s.cfg.DedupTTL).UTC().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(`DELETE FROM processed_events WHERE created_at < ?`, cutoff)
	return err
}

func (s *Server) resolveTelegramChatID(user UserData) (string, bool) {
	if value := rawToString(user.TelegramID); value != "" && value != "null" {
		return value, true
	}
	if s.cfg.ResolverDescriptionEnabled {
		matches := s.loginExpr.FindStringSubmatch(user.Description)
		if len(matches) == 2 {
			return matches[1], true
		}
	}
	return "", false
}

func (s *Server) sendTelegram(chatID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.cfg.TelegramBotToken)
	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", text)
	if s.cfg.TelegramParseMode != "" {
		form.Set("parse_mode", s.cfg.TelegramParseMode)
	}
	form.Set("disable_web_page_preview", "true")

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(apiURL, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("telegram api status=%d body=%s", resp.StatusCode, string(respBody))
	}
	return nil
}

func buildUserMessage(event RemnawaveEvent) string {
	duration := event.Data.Report.ActionReport.BlockDuration
	if duration <= 0 {
		duration = 60
	}
	return fmt.Sprintf("⚠️ VPN временно ограничил соединение\n\nМы обнаружили torrent/P2P-трафик через VPN. Такие подключения запрещены правилами сервиса, потому что они создают риск блокировок и проблем для всех пользователей.\n\nДоступ будет автоматически восстановлен примерно через %d секунд.\n\nПожалуйста, отключите torrent-клиент, раздачи, DHT/peer discovery и повторите подключение позже.", duration)
}

func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n json.Number
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&n); err == nil {
		return n.String()
	}
	return strings.TrimSpace(string(raw))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
