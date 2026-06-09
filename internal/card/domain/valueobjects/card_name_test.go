package valueobjects_test

import (
	"strings"
	"testing"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

func TestNewCardName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "valid 1 char", input: "A", wantErr: nil},
		{name: "valid 64 chars", input: strings.Repeat("x", 64), wantErr: nil},
		{name: "valid typical name", input: "Nubank Gold", wantErr: nil},
		{name: "empty string", input: "", wantErr: domain.ErrInvalidCardName},
		{name: "65 chars exceeds limit", input: strings.Repeat("x", 65), wantErr: domain.ErrInvalidCardName},
		{name: "100 chars exceeds limit", input: strings.Repeat("y", 100), wantErr: domain.ErrInvalidCardName},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := valueobjects.NewCardName(tc.input)
			if err != tc.wantErr {
				t.Errorf("NewCardName(%q): got err %v, want %v", tc.input, err, tc.wantErr)
			}
			if err == nil && got.String() != tc.input {
				t.Errorf("String(): got %q, want %q", got.String(), tc.input)
			}
		})
	}
}
