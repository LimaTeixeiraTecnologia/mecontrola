package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type fakeCheckoutUoW struct{}

func (u *fakeCheckoutUoW) Do(ctx context.Context, fn func(context.Context, database.DBTX) (entities.MagicToken, error), _ ...uow.Option) (entities.MagicToken, error) {
	return fn(ctx, nil)
}

type fakeCheckoutRepo struct {
	inserted entities.MagicToken
}

func (r *fakeCheckoutRepo) Insert(_ context.Context, token entities.MagicToken) error {
	r.inserted = token
	return nil
}
func (r *fakeCheckoutRepo) FindByHash(_ context.Context, _ []byte) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *fakeCheckoutRepo) FindPaidByMobileForFallback(_ context.Context, _ string) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *fakeCheckoutRepo) FindPaidForOutreach(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *fakeCheckoutRepo) UpdateMarkPaid(_ context.Context, _ entities.MagicToken) error { return nil }
func (r *fakeCheckoutRepo) UpdateMarkConsumed(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *fakeCheckoutRepo) UpdateMarkOutreachSent(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (r *fakeCheckoutRepo) UpdateMarkOutreachReset(_ context.Context, _ string) error { return nil }
func (r *fakeCheckoutRepo) BulkExpire(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *fakeCheckoutRepo) CountPaidUnconsumed(_ context.Context) (int64, error) { return 0, nil }

type fakeCheckoutFactory struct {
	repo appinterfaces.MagicTokenRepository
}

func (f *fakeCheckoutFactory) MagicTokenRepository(_ database.DBTX) appinterfaces.MagicTokenRepository {
	return f.repo
}
func (f *fakeCheckoutFactory) SupportSignalRepository(_ database.DBTX) appinterfaces.SupportSignalRepository {
	return nil
}
func (f *fakeCheckoutFactory) MetaMessageRepository(_ database.DBTX) appinterfaces.MetaMessageRepository {
	return nil
}
func (f *fakeCheckoutFactory) OnboardingCleanupRepository(_ database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return nil
}

type fakeCheckoutURLBuilder struct {
	token string
}

func (b *fakeCheckoutURLBuilder) Build(_ context.Context, _ string, token string) (string, error) {
	b.token = token
	return "https://pay.kiwify.com.br/checkout?sck=" + token, nil
}

type fakeCheckoutCipher struct {
	clear string
}

func (c *fakeCheckoutCipher) Encrypt(_ context.Context, clearToken string) (string, error) {
	c.clear = clearToken
	return "cipher:" + clearToken, nil
}

func (c *fakeCheckoutCipher) Decrypt(_ context.Context, ciphertext string) (string, error) {
	return ciphertext, nil
}

func TestCreateCheckoutSession_PersistsEncryptedTokenForOutreach(t *testing.T) {
	repo := &fakeCheckoutRepo{}
	builder := &fakeCheckoutURLBuilder{}
	cipher := &fakeCheckoutCipher{}
	uc := NewCreateCheckoutSession(
		&fakeCheckoutUoW{},
		&fakeCheckoutFactory{repo: repo},
		builder,
		cipher,
		id.NewUUIDGenerator(),
		7*24*time.Hour,
		noop.NewProvider(),
	)

	out, err := uc.Execute(context.Background(), input.CreateCheckoutSessionInput{PlanID: "plan-1"})

	require.NoError(t, err)
	require.NotEmpty(t, out.CheckoutURL)
	require.Equal(t, builder.token, cipher.clear)
	require.Equal(t, "cipher:"+builder.token, repo.inserted.ActivationTokenCiphertext())
	require.NotEqual(t, builder.token, repo.inserted.ActivationTokenCiphertext())
}
