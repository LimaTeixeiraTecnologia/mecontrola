package postgres_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	repopostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
)

func TestNewUserRepository_ReturnsNonNil(t *testing.T) {
	o11y := noop.NewProvider()
	repo := repopostgres.NewUserRepository(o11y, nil)
	assert.NotNil(t, repo)
}

func TestErrorSentinels_CanBeCheckedWithErrorsIs(t *testing.T) {
	wrapped := errors.New("identity.repository.user: " + application.ErrUserNotFound.Error())
	assert.False(t, errors.Is(wrapped, application.ErrUserNotFound))

	wrapped2 := errors.Join(application.ErrUserNotFound)
	assert.True(t, errors.Is(wrapped2, application.ErrUserNotFound))

	wrapped3 := errors.Join(application.ErrWhatsAppNumberInUse)
	assert.True(t, errors.Is(wrapped3, application.ErrWhatsAppNumberInUse))

	wrapped4 := errors.Join(application.ErrEmailInUse)
	assert.True(t, errors.Is(wrapped4, application.ErrEmailInUse))
}
