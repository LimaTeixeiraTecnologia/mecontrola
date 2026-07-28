package guards

import (
	"context"
	"regexp"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

var (
	whatsappFormatHeaderPattern = regexp.MustCompile(`(?m)^[ \t]*#{1,6}[ \t]*`)
	whatsappFormatBulletPattern = regexp.MustCompile(`(?m)^([ \t]*)-[ \t]+`)
)

type whatsappFormatSanitizerGuard struct{}

func NewWhatsappFormatSanitizerGuard() PostGuard {
	return &whatsappFormatSanitizerGuard{}
}

func (g *whatsappFormatSanitizerGuard) Name() string {
	return "whatsapp_format_sanitizer"
}

func (g *whatsappFormatSanitizerGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	sanitized := sanitizeWhatsappFormat(out.Content)
	if sanitized == out.Content {
		return GuardDecision{}
	}
	forced := out
	forced.Content = sanitized
	return GuardDecision{Handled: true, Result: forced}
}

func sanitizeWhatsappFormat(content string) string {
	sanitized := whatsappFormatHeaderPattern.ReplaceAllString(content, "")
	sanitized = whatsappFormatBulletPattern.ReplaceAllString(sanitized, "$1")
	sanitized = strings.ReplaceAll(sanitized, "**", "*")
	return sanitized
}
