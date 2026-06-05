package valueobjects

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
)

var ErrEmailInvalid = errors.New("identity: email invalid")

type Email struct {
	addr string
}

func NewEmail(raw string) (Email, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return Email{}, fmt.Errorf("identity: %w", ErrEmailInvalid)
	}
	if _, err := mail.ParseAddress(trimmed); err != nil {
		return Email{}, fmt.Errorf("identity: %q: %w", raw, ErrEmailInvalid)
	}
	return Email{addr: trimmed}, nil
}

func (e Email) String() string { return e.addr }

func (e Email) Equal(o Email) bool { return e.addr == o.addr }

func (e Email) Masked() string {
	at := strings.IndexByte(e.addr, '@')
	if at <= 0 {
		return "***"
	}
	return string([]rune(e.addr)[0:1]) + "***" + e.addr[at:]
}
