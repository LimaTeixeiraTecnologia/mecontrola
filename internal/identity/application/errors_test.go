package application_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
)

func TestSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{
		application.ErrUserNotFound,
		application.ErrWhatsAppNumberInUse,
		application.ErrEmailInUse,
		application.ErrEntitlementNotFound,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Fatalf("sentinels [%d]=%v and [%d]=%v unexpectedly equal", i, a, j, b)
			}
		}
	}
}

func TestErrEntitlementNotFoundIsMatched(t *testing.T) {
	wrapped := fmt.Errorf("identity.repository.entitlement.find_by_user_id: %w", application.ErrEntitlementNotFound)
	if !errors.Is(wrapped, application.ErrEntitlementNotFound) {
		t.Fatal("errors.Is must match the sentinel through %w wrapping")
	}
}

func TestErrUserNotFoundIsMatched(t *testing.T) {
	wrapped := fmt.Errorf("ctx: %w", application.ErrUserNotFound)
	if !errors.Is(wrapped, application.ErrUserNotFound) {
		t.Fatal("errors.Is must match ErrUserNotFound through %w wrapping")
	}
}
