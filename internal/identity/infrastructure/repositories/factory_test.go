package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)

func TestNewRepositoryFactory_ReturnsNonNil(t *testing.T) {
	o11y := noop.NewProvider()
	factory := repositories.NewRepositoryFactory(o11y)
	assert.NotNil(t, factory)
}

func TestRepositoryFactory_UserRepository_ReturnsInterface(t *testing.T) {
	o11y := noop.NewProvider()
	factory := repositories.NewRepositoryFactory(o11y)

	repo := factory.UserRepository(nil)
	assert.NotNil(t, repo)

	_ = repo
}
