package httpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func verifyRemnawaveRequest(r *http.Request, body []byte, secret string, maxClockSkew time.Duration) error {
	signature := strings.TrimSpace(r.Header.Get("X-Remnawave-Signature"))
	if signature == "" {
		return errors.New("missing X-Remnawave-Signature")
	}

	mac := hmac.New(sha256.New, []byte(secret))
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
	if age < -maxClockSkew || age > maxClockSkew {
		return fmt.Errorf("timestamp outside allowed skew: %s", age)
	}

	return nil
}
