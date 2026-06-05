package outbox_test

import (
	"testing"

	dbmocks "github.com/JailtonJunior94/devkit-go/pkg/database/mocks"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

func TestNewRepositoryFactory_OutboxRepository_NaoNil(t *testing.T) {
	pool := dbmocks.NewMockDBTX(t)
	factory := outbox.NewRepositoryFactory(nil)
	repo := factory.OutboxRepository(pool)
	assert.NotNil(t, repo)
}
