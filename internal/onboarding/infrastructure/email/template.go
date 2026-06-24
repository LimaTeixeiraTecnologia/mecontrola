package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"sync"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
)

//go:embed templates/*.html.tmpl
var templatesFS embed.FS

type ActivationTemplate struct {
	once sync.Once
	tmpl *template.Template
	err  error
}

func NewActivationTemplate() *ActivationTemplate {
	return &ActivationTemplate{}
}

func (t *ActivationTemplate) Render(in usecases.ActivationTemplateInput) (string, string, error) {
	t.once.Do(func() {
		parsed, err := template.ParseFS(templatesFS, "templates/activation.html.tmpl")
		if err != nil {
			t.err = fmt.Errorf("email: parse activation template: %w", err)
			return
		}
		t.tmpl = parsed
	})
	if t.err != nil {
		return "", "", t.err
	}

	var buf bytes.Buffer
	if err := t.tmpl.Execute(&buf, in); err != nil {
		return "", "", fmt.Errorf("email: render activation template: %w", err)
	}

	html := buf.String()
	text := fmt.Sprintf(
		"Bem-vindo(a) ao MeControla!\n\nAtive sua conta abrindo este link no celular:\n%s\n\nEste link expira em %d horas.",
		in.WaMeURL,
		in.ExpiresInHours,
	)
	return html, text, nil
}
