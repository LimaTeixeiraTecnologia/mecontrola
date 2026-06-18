package consumers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type recordingUpsertExpense struct {
	inputs []input.UpsertExpenseInput
	err    error
}

func (f *recordingUpsertExpense) Execute(_ context.Context, in input.UpsertExpenseInput) (output.ExpenseOutput, error) {
	f.inputs = append(f.inputs, in)
	return output.ExpenseOutput{}, f.err
}

type cardPurchaseInstallmentFixture struct {
	itemID      string
	refMonth    string
	amountCents int64
	index       int
}

type cardPurchaseCreatedConsumerSuite struct {
	suite.Suite
}

func TestCardPurchaseCreatedConsumer(t *testing.T) {
	suite.Run(t, new(cardPurchaseCreatedConsumerSuite))
}

func (s *cardPurchaseCreatedConsumerSuite) buildEnvelope(aggregateID, userID, subcategoryID string, installments []cardPurchaseInstallmentFixture) outbox.Envelope {
	insts := make([]map[string]any, len(installments))
	for i, inst := range installments {
		itemID := inst.itemID
		if itemID == "" {
			itemID = uuid.New().String()
		}
		insts[i] = map[string]any{
			"item_id":      itemID,
			"ref_month":    inst.refMonth,
			"amount_cents": inst.amountCents,
			"index":        inst.index,
		}
	}
	raw, _ := json.Marshal(map[string]any{
		"aggregate_id":   aggregateID,
		"user_id":        userID,
		"occurred_at":    time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		"subcategory_id": subcategoryID,
		"installments":   insts,
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *cardPurchaseCreatedConsumerSuite) TestInstallments_CreateOneExpensePerInstallment() {
	aggregateID := uuid.New().String()
	userID := uuid.New().String()
	subcategoryID := uuid.New().String()
	installments := []cardPurchaseInstallmentFixture{
		{itemID: uuid.New().String(), refMonth: "2026-06", amountCents: 20000, index: 1},
		{itemID: uuid.New().String(), refMonth: "2026-07", amountCents: 20000, index: 2},
		{itemID: uuid.New().String(), refMonth: "2026-08", amountCents: 20000, index: 3},
	}
	env := s.buildEnvelope(aggregateID, userID, subcategoryID, installments)

	upsert := &recordingUpsertExpense{}
	consumer := consumers.NewCardPurchaseCreatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.card_purchase.created.v1", payload: env})
	s.Require().NoError(err)
	s.Require().Len(upsert.inputs, 3)

	for i, in := range upsert.inputs {
		s.Equal(userID, in.UserID)
		s.Equal("transactions_card", in.Source)
		s.Equal(subcategoryID, in.SubcategoryID)
		s.Equal(installments[i].refMonth, in.Competence)
		s.Equal(installments[i].amountCents, in.AmountCents)
		s.Equal(installments[i].itemID, in.ExternalTransactionID)
		_, voErr := valueobjects.NewExternalTransactionID(in.ExternalTransactionID)
		s.Require().NoError(voErr)
	}
}

func (s *cardPurchaseCreatedConsumerSuite) TestExternalIDIsUniquePerInstallment() {
	aggregateID := uuid.New().String()
	env := s.buildEnvelope(aggregateID, uuid.New().String(), uuid.New().String(), []cardPurchaseInstallmentFixture{
		{refMonth: "2026-06", amountCents: 100, index: 1},
		{refMonth: "2026-07", amountCents: 100, index: 2},
	})

	upsert := &recordingUpsertExpense{}
	consumer := consumers.NewCardPurchaseCreatedConsumer(upsert, noop.NewProvider())

	s.Require().NoError(consumer.Handle(context.Background(), stubEvent{eventType: "transactions.card_purchase.created.v1", payload: env}))
	s.Require().Len(upsert.inputs, 2)
	s.NotEqual(upsert.inputs[0].ExternalTransactionID, upsert.inputs[1].ExternalTransactionID)
}

func (s *cardPurchaseCreatedConsumerSuite) TestMissingSubcategory_NoOp() {
	env := s.buildEnvelope(uuid.New().String(), uuid.New().String(), uuid.Nil.String(), []cardPurchaseInstallmentFixture{
		{refMonth: "2026-06", amountCents: 100, index: 1},
	})

	upsert := &recordingUpsertExpense{}
	consumer := consumers.NewCardPurchaseCreatedConsumer(upsert, noop.NewProvider())

	s.Require().NoError(consumer.Handle(context.Background(), stubEvent{eventType: "transactions.card_purchase.created.v1", payload: env}))
	s.Empty(upsert.inputs)
}

func (s *cardPurchaseCreatedConsumerSuite) TestEmptySubcategory_NoOp() {
	env := s.buildEnvelope(uuid.New().String(), uuid.New().String(), "", []cardPurchaseInstallmentFixture{
		{refMonth: "2026-06", amountCents: 100, index: 1},
	})

	upsert := &recordingUpsertExpense{}
	consumer := consumers.NewCardPurchaseCreatedConsumer(upsert, noop.NewProvider())

	s.Require().NoError(consumer.Handle(context.Background(), stubEvent{eventType: "transactions.card_purchase.created.v1", payload: env}))
	s.Empty(upsert.inputs)
}

func (s *cardPurchaseCreatedConsumerSuite) TestRedelivery_StableExternalIDs() {
	aggregateID := uuid.New().String()
	env := s.buildEnvelope(aggregateID, uuid.New().String(), uuid.New().String(), []cardPurchaseInstallmentFixture{
		{refMonth: "2026-06", amountCents: 100, index: 1},
		{refMonth: "2026-07", amountCents: 100, index: 2},
	})

	upsert := &recordingUpsertExpense{}
	consumer := consumers.NewCardPurchaseCreatedConsumer(upsert, noop.NewProvider())
	evt := stubEvent{eventType: "transactions.card_purchase.created.v1", payload: env}

	s.Require().NoError(consumer.Handle(context.Background(), evt))
	s.Require().NoError(consumer.Handle(context.Background(), evt))
	s.Require().Len(upsert.inputs, 4)
	s.Equal(upsert.inputs[0].ExternalTransactionID, upsert.inputs[2].ExternalTransactionID)
	s.Equal(upsert.inputs[1].ExternalTransactionID, upsert.inputs[3].ExternalTransactionID)
}

func (s *cardPurchaseCreatedConsumerSuite) TestTombstoneConflict_SkippedNoError() {
	env := s.buildEnvelope(uuid.New().String(), uuid.New().String(), uuid.New().String(), []cardPurchaseInstallmentFixture{
		{refMonth: "2026-06", amountCents: 100, index: 1},
	})

	upsert := &recordingUpsertExpense{err: appinterfaces.ErrExpenseTombstoneConflict}
	consumer := consumers.NewCardPurchaseCreatedConsumer(upsert, noop.NewProvider())

	s.Require().NoError(consumer.Handle(context.Background(), stubEvent{eventType: "transactions.card_purchase.created.v1", payload: env}))
	s.Require().Len(upsert.inputs, 1)
}

func (s *cardPurchaseCreatedConsumerSuite) TestUpsertError_Propagated() {
	env := s.buildEnvelope(uuid.New().String(), uuid.New().String(), uuid.New().String(), []cardPurchaseInstallmentFixture{
		{refMonth: "2026-06", amountCents: 100, index: 1},
	})

	upsert := &recordingUpsertExpense{err: appinterfaces.ErrExpenseConflict}
	consumer := consumers.NewCardPurchaseCreatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.card_purchase.created.v1", payload: env})
	s.Require().Error(err)
}

func (s *cardPurchaseCreatedConsumerSuite) TestInvalidPayloadType_ReturnsError() {
	consumer := consumers.NewCardPurchaseCreatedConsumer(&recordingUpsertExpense{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.card_purchase.created.v1", payload: "not-envelope"})
	s.Require().Error(err)
}
