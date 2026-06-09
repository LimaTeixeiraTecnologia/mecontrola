package valueobjects_test

import (
	"strings"
	"testing"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

func TestNewNickname(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "valid 1 char", input: "A", wantErr: nil},
		{name: "valid 32 chars", input: strings.Repeat("n", 32), wantErr: nil},
		{name: "valid typical", input: "nubank-ouro", wantErr: nil},
		{name: "empty string", input: "", wantErr: domain.ErrInvalidNickname},
		{name: "33 chars exceeds limit", input: strings.Repeat("n", 33), wantErr: domain.ErrInvalidNickname},
		{name: "64 chars exceeds limit", input: strings.Repeat("n", 64), wantErr: domain.ErrInvalidNickname},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := valueobjects.NewNickname(tc.input)
			if err != tc.wantErr {
				t.Errorf("NewNickname(%q): got err %v, want %v", tc.input, err, tc.wantErr)
			}
			if err == nil && got.String() != tc.input {
				t.Errorf("String(): got %q, want %q", got.String(), tc.input)
			}
		})
	}
}
