package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/ryabkov82/vff-remnawave-events/internal/config"
	"github.com/ryabkov82/vff-remnawave-events/internal/dedup"
	"github.com/ryabkov82/vff-remnawave-events/internal/notify"
	"github.com/ryabkov82/vff-remnawave-events/internal/remnawave"
	"github.com/ryabkov82/vff-remnawave-events/internal/resolver"
	"github.com/ryabkov82/vff-remnawave-events/internal/shm"
	"github.com/ryabkov82/vff-remnawave-events/internal/telegram"
)

type RecipientResolver interface {
	Resolve(user remnawave.UserData) (string, bool)
}

type EventMessenger interface {
	SendEventMessage(ctx context.Context, event remnawave.Event, chatID, text string) error
}

type Server struct {
	cfg       config.Config
	store     *dedup.Store
	resolver  RecipientResolver
	messenger EventMessenger
}

func New(cfg config.Config, store *dedup.Store, recipientResolver RecipientResolver, messenger EventMessenger) *Server {
	return &Server{
		cfg:       cfg,
		store:     store,
		resolver:  recipientResolver,
		messenger: messenger,
	}
}

func NewDefault(cfg config.Config, store *dedup.Store) *Server {
	return New(
		cfg,
		store,
		resolver.NewDescriptionResolver(cfg.ResolverDescriptionEnabled),
		newMessenger(cfg),
	)
}

func newMessenger(cfg config.Config) EventMessenger {
	var categoryResolver telegram.CategoryResolver
	if cfg.SHMAdminBaseURL != "" {
		categoryResolver = shm.NewClient(cfg.SHMAdminBaseURL, cfg.SHMAdminAuthHeaderName, cfg.SHMAdminAuthHeaderValue, cfg.SHMRequestTimeout)
	}
	return telegram.NewRoutedClient(cfg.TelegramBotToken, cfg.TelegramParseMode, categoryResolver, cfg.TelegramBotTokensByCategory, cfg.MessengerDryRun)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /remnawave", s.remnawave)
	return mux
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

	if err := verifyRemnawaveRequest(r, body, s.cfg.WebhookSecretHeader, s.cfg.WebhookMaxClockSkew); err != nil {
		log.Printf("invalid webhook request: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	event, err := remnawave.ParseEvent(body)
	if err != nil {
		log.Printf("invalid json: %v", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if !remnawave.IsTorrentBlockEvent(event) {
		log.Printf("ignored event scope=%s event=%s", event.Scope, event.Event)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	if err := s.store.DeleteOlderThan(s.cfg.DedupTTL); err != nil {
		log.Printf("dedup ttl cleanup failed: %v", err)
	}

	eventKey := remnawave.DedupKey(event)
	inserted, err := s.store.InsertEvent(eventKey, event)
	if err != nil {
		log.Printf("dedup insert failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !inserted {
		log.Printf("duplicate torrent blocker event key=%s user=%s node=%s", eventKey, remnawave.RawToString(event.Data.User.ID), event.Data.Node.Name)
		writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
		return
	}

	chatID, ok := s.resolver.Resolve(event.Data.User)
	if !ok {
		log.Printf("telegram chat id not found user_id=%s username=%s description=%q", remnawave.RawToString(event.Data.User.ID), event.Data.User.Username, event.Data.User.Description)
		writeJSON(w, http.StatusOK, map[string]string{"status": "no_recipient"})
		return
	}

	if err := s.messenger.SendEventMessage(r.Context(), event, chatID, notify.TorrentBlockMessage(event)); err != nil {
		log.Printf("telegram send failed chat_id=%s user_id=%s error=%v", chatID, remnawave.RawToString(event.Data.User.ID), err)
		writeJSON(w, http.StatusOK, map[string]string{"status": "telegram_failed"})
		return
	}

	log.Printf("telegram notification sent chat_id=%s user_id=%s username=%s node=%s", chatID, remnawave.RawToString(event.Data.User.ID), event.Data.User.Username, event.Data.Node.Name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
