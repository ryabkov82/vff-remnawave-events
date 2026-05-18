package shm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL         string
	authHeaderName  string
	authHeaderValue string
	http            *http.Client
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

func NewClient(baseURL, authHeaderName, authHeaderValue string, timeout time.Duration) *Client {
	return &Client{
		baseURL:         baseURL,
		authHeaderName:  authHeaderName,
		authHeaderValue: authHeaderValue,
		http:            &http.Client{Timeout: timeout},
	}
}

func (c *Client) ServiceCategory(ctx context.Context, userServiceID int) (string, error) {
	filter := fmt.Sprintf(`{"user_service_id":%d}`, userServiceID)
	endpoint := c.baseURL + "/shm/v1/admin/user/service?filter=" + url.QueryEscape(filter)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	if c.authHeaderName != "" {
		req.Header.Set(c.authHeaderName, c.authHeaderValue)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("shm api status=%d", resp.StatusCode)
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
