//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status"
	statuspostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status/postgres"
)

type DeliveryCountsIntegrationSuite struct {
	suite.Suite
	ctx    context.Context
	db     *sqlx.DB
	lookup *status.LookupDeliveryState
}

func TestDeliveryCountsIntegrationSuite(t *testing.T) {
	suite.Run(t, new(DeliveryCountsIntegrationSuite))
}

func (s *DeliveryCountsIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
	reader := statuspostgres.NewMessageStatusRepository(fake.NewProvider(), s.db).(status.MessageStatusReader)
	s.lookup = status.NewLookupDeliveryState(reader, fake.NewProvider())
}

func (s *DeliveryCountsIntegrationSuite) insertStatus(messageID, deliveryStatus string) {
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.whatsapp_message_status
			(id, message_id, status, recipient_id, status_at, created_at)
		VALUES ($1,$2,$3,'5511',$4,now())`,
		uuid.New(), messageID, deliveryStatus, time.Now().UTC(),
	)
	s.Require().NoError(err)
}

func (s *DeliveryCountsIntegrationSuite) TestNotReceived() {
	state, err := s.lookup.Execute(s.ctx, "wamid-none-"+uuid.NewString())
	s.Require().NoError(err)
	s.Equal(status.DeliveryStateNotReceived, state)
}

func (s *DeliveryCountsIntegrationSuite) TestFailed() {
	messageID := "wamid-failed-" + uuid.NewString()
	s.insertStatus(messageID, "sent")
	s.insertStatus(messageID, "failed")

	state, err := s.lookup.Execute(s.ctx, messageID)
	s.Require().NoError(err)
	s.Equal(status.DeliveryStateFailed, state)
}

func (s *DeliveryCountsIntegrationSuite) TestDelivered() {
	messageID := "wamid-delivered-" + uuid.NewString()
	s.insertStatus(messageID, "sent")
	s.insertStatus(messageID, "delivered")

	state, err := s.lookup.Execute(s.ctx, messageID)
	s.Require().NoError(err)
	s.Equal(status.DeliveryStateDelivered, state)
}
