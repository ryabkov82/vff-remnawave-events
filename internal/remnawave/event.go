package remnawave

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// Event is the top-level Remnawave webhook payload.
type Event struct {
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

func ParseEvent(body []byte) (Event, error) {
	var event Event
	err := json.Unmarshal(body, &event)
	return event, err
}

func IsTorrentBlockEvent(event Event) bool {
	return event.Scope == "torrent_blocker" &&
		event.Event == "torrent_blocker.report" &&
		event.Data.Report.ActionReport.Blocked
}

func DedupKey(event Event) string {
	parts := []string{
		event.Scope,
		event.Event,
		RawToString(event.Data.User.ID),
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

func RawToString(raw json.RawMessage) string {
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
