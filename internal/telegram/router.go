package telegram

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/ryabkov82/vff-remnawave-events/internal/remnawave"
)

type CategoryResolver interface {
	ServiceCategory(ctx context.Context, userServiceID int) (string, error)
}

type RoutedClient struct {
	defaultSecret string
	parseMode     string
	resolver      CategoryResolver
	byCategory    map[string]string
	dryRun        bool
	usernameExpr  *regexp.Regexp
}

func NewRoutedClient(defaultSecret, parseMode string, resolver CategoryResolver, byCategory map[string]string, dryRun bool) *RoutedClient {
	return &RoutedClient{
		defaultSecret: defaultSecret,
		parseMode:     parseMode,
		resolver:      resolver,
		byCategory:    byCategory,
		dryRun:        dryRun,
		usernameExpr:  regexp.MustCompile(`^us_([0-9]+)$`),
	}
}

func (c *RoutedClient) SendEventMessage(ctx context.Context, event remnawave.Event, chatID, text string) error {
	if c.dryRun {
		return NewDryRunClient().SendMessage(chatID, text)
	}

	userServiceID, ok := c.userServiceID(event.Data.User.Username)
	if !ok || c.resolver == nil || len(c.byCategory) == 0 {
		return NewClient(c.defaultSecret, c.parseMode).SendMessage(chatID, text)
	}

	category, err := c.resolver.ServiceCategory(ctx, userServiceID)
	if err != nil {
		if c.defaultSecret == "" {
			return fmt.Errorf("resolve SHM service category failed and no default messenger is configured: %w", err)
		}
		log.Printf("SHM service category lookup failed user_service_id=%d username=%s error=%v; using default messenger", userServiceID, event.Data.User.Username, err)
		return NewClient(c.defaultSecret, c.parseMode).SendMessage(chatID, text)
	}

	secret, ok := c.byCategory[category]
	if !ok || secret == "" {
		if c.defaultSecret == "" {
			return fmt.Errorf("no messenger configured for service category=%s and no default messenger is configured", category)
		}
		log.Printf("no messenger configured for service category=%s user_service_id=%d username=%s; using default messenger", category, userServiceID, event.Data.User.Username)
		return NewClient(c.defaultSecret, c.parseMode).SendMessage(chatID, text)
	}

	log.Printf("messenger route category=%s user_service_id=%d username=%s", category, userServiceID, event.Data.User.Username)
	return NewClient(secret, c.parseMode).SendMessage(chatID, text)
}

func (c *RoutedClient) SendMessage(chatID, text string) error {
	if c.dryRun {
		return NewDryRunClient().SendMessage(chatID, text)
	}
	return NewClient(c.defaultSecret, c.parseMode).SendMessage(chatID, text)
}

func (c *RoutedClient) userServiceID(username string) (int, bool) {
	matches := c.usernameExpr.FindStringSubmatch(username)
	if len(matches) != 2 {
		return 0, false
	}
	id, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	return id, true
}
