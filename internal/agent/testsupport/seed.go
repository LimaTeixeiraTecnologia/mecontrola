package testsupport

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)

func SeedActiveUserWA(t *testing.T, mgr manager.Manager, waNumber string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	o11y := noop.NewProvider()
	factory := identityrepos.NewRepositoryFactory(o11y)
	db := mgr.DBTX(ctx)

	wa, err := valueobjects.NewWhatsAppNumber(waNumber)
	if err != nil {
		t.Fatalf("testsupport.seed: invalid wa number %q: %v", waNumber, err)
	}
	candidate := entities.New(wa)
	user, err := factory.UserRepository(db).UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	if err != nil {
		t.Fatalf("testsupport.seed: upsert user: %v", err)
	}
	userID, err := uuid.Parse(user.ID())
	if err != nil {
		t.Fatalf("testsupport.seed: parse user id: %v", err)
	}

	entitlement := interfaces.EntitlementRecord{
		UserID:         userID.String(),
		SubscriptionID: uuid.New().String(),
		Status:         "ACTIVE",
		PeriodEnd:      time.Now().UTC().Add(365 * 24 * time.Hour),
	}
	if upsertErr := factory.EntitlementRepository(db).Upsert(ctx, entitlement); upsertErr != nil {
		t.Fatalf("testsupport.seed: upsert entitlement: %v", upsertErr)
	}
	return userID
}
