package valueobjects_test

import (
	"testing"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

func TestNewBillingCycle(t *testing.T) {
	cases := []struct {
		name       string
		closingDay int
		dueDay     int
		wantErr    error
	}{
		{name: "valid 1-1", closingDay: 1, dueDay: 1, wantErr: nil},
		{name: "valid 31-31", closingDay: 31, dueDay: 31, wantErr: nil},
		{name: "valid 20-5", closingDay: 20, dueDay: 5, wantErr: nil},
		{name: "valid 5-20", closingDay: 5, dueDay: 20, wantErr: nil},
		{name: "valid 15-15", closingDay: 15, dueDay: 15, wantErr: nil},
		{name: "closing 0 invalid", closingDay: 0, dueDay: 10, wantErr: domain.ErrInvalidClosingDay},
		{name: "closing 32 invalid", closingDay: 32, dueDay: 10, wantErr: domain.ErrInvalidClosingDay},
		{name: "closing negative", closingDay: -1, dueDay: 10, wantErr: domain.ErrInvalidClosingDay},
		{name: "due 0 invalid", closingDay: 10, dueDay: 0, wantErr: domain.ErrInvalidDueDay},
		{name: "due 32 invalid", closingDay: 10, dueDay: 32, wantErr: domain.ErrInvalidDueDay},
		{name: "due negative", closingDay: 10, dueDay: -5, wantErr: domain.ErrInvalidDueDay},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := valueobjects.NewBillingCycle(tc.closingDay, tc.dueDay)
			if err != tc.wantErr {
				t.Errorf("NewBillingCycle(%d, %d): got err %v, want %v", tc.closingDay, tc.dueDay, err, tc.wantErr)
			}
			if err == nil {
				if got.ClosingDay != tc.closingDay {
					t.Errorf("ClosingDay: got %d, want %d", got.ClosingDay, tc.closingDay)
				}
				if got.DueDay != tc.dueDay {
					t.Errorf("DueDay: got %d, want %d", got.DueDay, tc.dueDay)
				}
			}
		})
	}
}
