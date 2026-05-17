package resolver

import (
	"regexp"

	"github.com/ryabkov82/vff-remnawave-events/internal/remnawave"
)

// DescriptionResolver extracts Telegram chat ID from Remnawave user fields.
type DescriptionResolver struct {
	enabled   bool
	loginExpr *regexp.Regexp
}

func NewDescriptionResolver(enabled bool) *DescriptionResolver {
	return &DescriptionResolver{
		enabled:   enabled,
		loginExpr: regexp.MustCompile(`login=@?([0-9]{5,20})`),
	}
}

func (r *DescriptionResolver) Resolve(user remnawave.UserData) (string, bool) {
	if value := remnawave.RawToString(user.TelegramID); value != "" && value != "null" {
		return value, true
	}

	if !r.enabled {
		return "", false
	}

	matches := r.loginExpr.FindStringSubmatch(user.Description)
	if len(matches) == 2 {
		return matches[1], true
	}

	return "", false
}
