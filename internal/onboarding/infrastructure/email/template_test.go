package email_test

import (
	"strings"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/email"
)

func TestActivationTemplateRendersRealTemplate(t *testing.T) {
	tmpl := email.NewActivationTemplate()

	in := usecases.ActivationTemplateInput{
		ActivationURL:  "https://mecontrola.app.br/ativar?token=abc123",
		SupportURL:     "https://wa.me/5511999999999",
		ExpiresInHours: 24,
	}

	html, text, err := tmpl.Render(in)
	if err != nil {
		t.Fatalf("render returned error: %v", err)
	}

	if !strings.Contains(html, in.ActivationURL) {
		t.Fatalf("html does not contain activation URL %q:\n%s", in.ActivationURL, html)
	}
	if !strings.Contains(html, in.SupportURL) {
		t.Fatalf("html does not contain support URL %q", in.SupportURL)
	}
	if strings.Contains(html, "wa.me/5511999999999?text=") || strings.Contains(html, "ATIVAR") {
		t.Fatalf("html exposes a direct wa.me activation link or legacy code:\n%s", html)
	}
	if !strings.Contains(text, in.ActivationURL) {
		t.Fatalf("text body does not contain activation URL")
	}
}
