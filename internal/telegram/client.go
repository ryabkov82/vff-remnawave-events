package telegram

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client sends Telegram Bot API messages.
type Client struct {
	botToken  string
	parseMode string
	http      *http.Client
}

func NewClient(botToken, parseMode string) *Client {
	return &Client{
		botToken:  botToken,
		parseMode: parseMode,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) SendMessage(chatID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.botToken)
	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", text)
	if c.parseMode != "" {
		form.Set("parse_mode", c.parseMode)
	}
	form.Set("disable_web_page_preview", "true")

	resp, err := c.http.PostForm(apiURL, form)
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
