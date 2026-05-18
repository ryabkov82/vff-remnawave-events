package shm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"
)

type Client struct {
	baseURL    string
	login      string
	password   string
	http       *http.Client
	sessionMu  sync.Mutex
	sessionID  string
}

type UserService struct {
	UserServiceID int    `json:"user_service_id"`
	Category      string `json:"category"`
	Services      struct {
		Category string `json:"category"`
	} `json:"services"`
}

type userServiceResponse struct {
	Data []UserService `json:"data"`
}

func NewClient(baseURL, login, password string, timeout time.Duration) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL:  baseURL,
		login:    login,
		password: password,
		http:     &http.Client{Timeout: timeout, Jar: jar},
	}
}

func (c *Client) ServiceCategory(ctx context.Context, userServiceID int) (string, error) {
	category, err := c.serviceCategory(ctx, userServiceID)
	if err == nil {
		return category, nil
	}

	if authErr := c.authenticate(ctx); authErr != nil {
		return "", fmt.Errorf("SHM auth failed after lookup error %v: %w", err, authErr)
	}

	return c.serviceCategory(ctx, userServiceID)
}

func (c *Client) authenticate(ctx context.Context) error {
	body, err := json.Marshal(map[string]string{
		"login":    c.login,
		"password": c.password,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/shm/user/auth.cgi", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("SHM auth status=%d", resp.StatusCode)
	}

	var parsed struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	if parsed.SessionID == "" {
		return fmt.Errorf("SHM auth returned empty session_id")
	}

	c.sessionMu.Lock()
	c.sessionID = parsed.SessionID
	c.sessionMu.Unlock()

	base, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	c.http.Jar.SetCookies(base, []*http.Cookie{{
		Name:    "session_id",
		Value:   parsed.SessionID,
		Path:    "/",
		Expires: time.Now().Add(24 * time.Hour),
	}})

	return nil
}

func (c *Client) serviceCategory(ctx context.Context, userServiceID int) (string, error) {
	filterBytes, err := json.Marshal(map[string]int{"user_service_id": userServiceID})
	if err != nil {
		return "", err
	}
	endpoint := c.baseURL + "/shm/v1/admin/user/service?filter=" + url.QueryEscape(string(filterBytes))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("SHM session rejected status=%d", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("SHM API status=%d", resp.StatusCode)
	}

	var parsed userServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Data) == 0 {
		return "", fmt.Errorf("user_service_id=%d not found", userServiceID)
	}

	category := parsed.Data[0].Services.Category
	if category == "" {
		category = parsed.Data[0].Category
	}
	if category == "" {
		return "", fmt.Errorf("user_service_id=%d has empty category", userServiceID)
	}

	return category, nil
}
