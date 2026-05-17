package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:8080/remnawave", "Webhook URL")
	secret := flag.String("secret", "change-me", "Webhook HMAC secret")
	payloadPath := flag.String("payload", "testdata/torrent_blocker_report.json", "Path to webhook payload JSON")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP client timeout")
	flag.Parse()

	body, timestamp, err := preparePayload(*payloadPath)
	if err != nil {
		log.Fatalf("prepare payload: %v", err)
	}

	signature := sign(body, *secret)

	req, err := http.NewRequest(http.MethodPost, *url, bytes.NewReader(body))
	if err != nil {
		log.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Remnawave-Signature", signature)
	req.Header.Set("X-Remnawave-Timestamp", timestamp)
	req.Header.Set("User-Agent", "Remnawave-test")

	client := http.Client{Timeout: *timeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	fmt.Printf("HTTP %d\n%s\n", resp.StatusCode, string(respBody))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		os.Exit(1)
	}
}

func preparePayload(path string) ([]byte, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, "", err
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	payload["timestamp"] = now

	setNestedString(payload, now, "data", "report", "actionReport", "processedAt")

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	return body, now, nil
}

func setNestedString(root map[string]any, value string, path ...string) {
	if len(path) == 0 {
		return
	}
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]any)
		if !ok {
			return
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
