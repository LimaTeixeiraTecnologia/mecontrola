package channels_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/channels"
)

func TestMatchActivationCommand(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantToken string
		wantOK    bool
	}{
		{name: "exact ATIVAR + 43 chars", input: "ATIVAR " + strings.Repeat("a", 43), wantToken: strings.Repeat("a", 43), wantOK: true},
		{name: "case insensitive", input: "ativar " + strings.Repeat("b", 40), wantToken: strings.Repeat("b", 40), wantOK: true},
		{name: "max 45 chars", input: "ATIVAR " + strings.Repeat("c", 45), wantToken: strings.Repeat("c", 45), wantOK: true},
		{name: "leading/trailing whitespace", input: "  ATIVAR " + strings.Repeat("d", 43) + "  ", wantToken: strings.Repeat("d", 43), wantOK: true},
		{name: "with underscores and dashes", input: "ATIVAR abc_def-ghi_jkl-mno_pqr-stu_vwx-yzA_BCD-EFG", wantToken: "abc_def-ghi_jkl-mno_pqr-stu_vwx-yzA_BCD-EFG", wantOK: true},
		{name: "too short (39 chars)", input: "ATIVAR " + strings.Repeat("a", 39), wantOK: false},
		{name: "too long (46 chars)", input: "ATIVAR " + strings.Repeat("a", 46), wantOK: false},
		{name: "missing ATIVAR", input: strings.Repeat("a", 43), wantOK: false},
		{name: "empty", input: "", wantOK: false},
		{name: "invalid characters", input: "ATIVAR token with spaces here xxxxxxxxxxxxxxxxxxxx", wantOK: false},
		{name: "regular message", input: "quanto gastei esse mes?", wantOK: false},
		{name: "telegram deep link 43 chars", input: "/start ATIVAR_" + strings.Repeat("a", 43), wantToken: strings.Repeat("a", 43), wantOK: true},
		{name: "telegram deep link 40 chars", input: "/start ATIVAR_" + strings.Repeat("b", 40), wantToken: strings.Repeat("b", 40), wantOK: true},
		{name: "telegram deep link 45 chars", input: "/start ATIVAR_" + strings.Repeat("c", 45), wantToken: strings.Repeat("c", 45), wantOK: true},
		{name: "telegram deep link case insensitive", input: "/START ativar_" + strings.Repeat("d", 43), wantToken: strings.Repeat("d", 43), wantOK: true},
		{name: "telegram deep link with whitespace", input: "  /start ATIVAR_" + strings.Repeat("e", 43) + "  ", wantToken: strings.Repeat("e", 43), wantOK: true},
		{name: "telegram deep link underscores and dashes", input: "/start ATIVAR_abc_def-ghi_jkl-mno_pqr-stu_vwx-yzA_BCD-EFG", wantToken: "abc_def-ghi_jkl-mno_pqr-stu_vwx-yzA_BCD-EFG", wantOK: true},
		{name: "telegram /start space ATIVAR space token rejected", input: "/start ATIVAR " + strings.Repeat("a", 43), wantOK: false},
		{name: "telegram /start without ATIVAR_ prefix rejected", input: "/start " + strings.Repeat("a", 43), wantOK: false},
		{name: "telegram deep link too short (39 chars)", input: "/start ATIVAR_" + strings.Repeat("a", 39), wantOK: false},
		{name: "telegram deep link too long (46 chars)", input: "/start ATIVAR_" + strings.Repeat("a", 46), wantOK: false},
		{name: "telegram /start alone rejected", input: "/start", wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, ok := channels.MatchActivationCommand(tc.input)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantToken, token)
			}
		})
	}
}

func TestActivationPattern_IsCanonical(t *testing.T) {
	pattern := channels.ActivationPattern()
	assert.NotNil(t, pattern)
	assert.Equal(t, `(?i)^\s*ATIVAR\s+([A-Za-z0-9_\-]{40,45})\s*$`, pattern.String(),
		"pattern canônico — não alterar sem migrar callers (WA dispatcher, TG dispatcher, WA wiring, TG wiring)")
}
