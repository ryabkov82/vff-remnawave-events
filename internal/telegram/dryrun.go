package telegram

import "log"

// DryRunClient logs messages without calling external Telegram API.
type DryRunClient struct{}

func NewDryRunClient() *DryRunClient {
	return &DryRunClient{}
}

func (c *DryRunClient) SendMessage(chatID, text string) error {
	log.Printf("messenger dry-run: chat_id=%s text=%q", chatID, text)
	return nil
}
