package valueobjects

import (
	"errors"
	"net/mail"
	"strings"
)

type Email struct{ value string }

var (
	ErrEmptyEmail   = errors.New("identity: email vazio")
	ErrInvalidEmail = errors.New("identity: email formato inválido")
)

type emailValidator struct{}

func (emailValidator) HasTLD(address string) bool {
	at := strings.LastIndex(address, "@")
	if at < 0 || at == len(address)-1 {
		return false
	}
	domain := address[at+1:]
	dot := strings.LastIndex(domain, ".")
	return dot > 0 && dot < len(domain)-1
}

func (v emailValidator) Parse(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", ErrEmptyEmail
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", ErrInvalidEmail
	}
	address := strings.ToLower(parsed.Address)
	if !strings.Contains(address, "@") || !v.HasTLD(address) {
		return "", ErrInvalidEmail
	}
	return address, nil
}

func NewEmail(input string) (Email, error) {
	address, err := emailValidator{}.Parse(input)
	if err != nil {
		return Email{}, err
	}
	return Email{value: address}, nil
}

func (e Email) String() string          { return e.value }
func (e Email) Equals(other Email) bool { return e.value == other.value }
func (e Email) IsZero() bool            { return e.value == "" }
