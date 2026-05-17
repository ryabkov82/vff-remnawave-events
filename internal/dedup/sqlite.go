package dedup

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ryabkov82/vff-remnawave-events/internal/remnawave"

	_ "github.com/mattn/go-sqlite3"
)

// Store tracks already processed webhook events.
type Store struct {
	db *sql.DB
}

func Open(sqlitePath string) (*Store, error) {
	if err := ensureParentDir(sqlitePath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) init() error {
	_, err := s.db.Exec(`
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

func (s *Store) InsertEvent(eventKey string, event remnawave.Event) (bool, error) {
	_, err := s.db.Exec(`
INSERT INTO processed_events (event_key, event_ts, remnawave_user_id, remnawave_username, node_name, source_ip)
VALUES (?, ?, ?, ?, ?, ?)
`, eventKey, event.Timestamp.Format(time.RFC3339Nano), remnawave.RawToString(event.Data.User.ID), event.Data.User.Username, event.Data.Node.Name, event.Data.Report.ActionReport.IP)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Store) DeleteOlderThan(ttl time.Duration) error {
	cutoff := time.Now().Add(-ttl).UTC().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(`DELETE FROM processed_events WHERE created_at < ?`, cutoff)
	return err
}

func ensureParentDir(path string) error {
	if path == "" || path == ":memory:" {
		return nil
	}

	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}
