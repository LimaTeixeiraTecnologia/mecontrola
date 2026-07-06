package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type TransactionWorkflowSuite struct {
	suite.Suite
	sut TransactionWorkflow
	now time.Time
}

func TestTransactionWorkflowSuite(t *testing.T) {
	suite.Run(t, new(TransactionWorkflowSuite))
}

func (s *TransactionWorkflowSuite) SetupTest() {
	s.sut = NewTransactionWorkflow()
	s.now = time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
}

func (s *TransactionWorkflowSuite) money(cents int64) valueobjects.Money {
	m, err := valueobjects.NewMoney(cents)
	s.Require().NoError(err)
	return m
}

func (s *TransactionWorkflowSuite) installmentCount(n int) valueobjects.InstallmentCount {
	ic, err := valueobjects.NewInstallmentCount(n)
	s.Require().NoError(err)
	return ic
}

func (s *TransactionWorkflowSuite) itemIDs(pm valueobjects.PaymentMethod, installments option.Option[valueobjects.InstallmentCount]) []uuid.UUID {
	if pm != valueobjects.PaymentMethodCreditCard {
		return nil
	}
	n := 1
	if ic, ok := installments.Get(); ok {
		n = ic.Value()
	}
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = uuid.New()
	}
	return ids
}

func (s *TransactionWorkflowSuite) createItemIDs(cmd commands.CreateTransaction) []uuid.UUID {
	return s.itemIDs(cmd.PaymentMethod, cmd.Installments)
}

func (s *TransactionWorkflowSuite) updateItemIDs(cmd commands.UpdateTransaction) []uuid.UUID {
	return s.itemIDs(cmd.PaymentMethod, cmd.Installments)
}

func (s *TransactionWorkflowSuite) snapshot(closing, due int) valueobjects.CardBillingSnapshot {
	snap, err := valueobjects.NewCardBillingSnapshot(closing, due)
	s.Require().NoError(err)
	return snap
}

func (s *TransactionWorkflowSuite) simpleCreateCmd(direction, method string, cents int64, occurredAt time.Time) commands.CreateTransaction {
	dir, err := valueobjects.ParseDirection(direction)
	s.Require().NoError(err)
	pm, err := valueobjects.ParsePaymentMethodForCreate(method)
	s.Require().NoError(err)
	desc, err := valueobjects.NewDescription("Test transaction")
	s.Require().NoError(err)
	return commands.CreateTransaction{
		UserID:        valueobjects.UserIDFromUUID(uuid.New()),
		Direction:     dir,
		PaymentMethod: pm,
		Amount:        s.money(cents),
		Description:   desc,
		CategoryID:    valueobjects.CategoryIDFromUUID(uuid.New()),
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		CardID:        option.None[valueobjects.CardID](),
		Installments:  option.None[valueobjects.InstallmentCount](),
		OccurredAt:    occurredAt,
	}
}

func (s *TransactionWorkflowSuite) cardCreateCmd(cents int64, installments int, occurredAt time.Time) commands.CreateTransaction {
	pm, err := valueobjects.ParsePaymentMethodForCreate("credit_card")
	s.Require().NoError(err)
	desc, err := valueobjects.NewDescription("Card purchase")
	s.Require().NoError(err)
	return commands.CreateTransaction{
		UserID:        valueobjects.UserIDFromUUID(uuid.New()),
		Direction:     valueobjects.DirectionOutcome,
		PaymentMethod: pm,
		Amount:        s.money(cents),
		Description:   desc,
		CategoryID:    valueobjects.CategoryIDFromUUID(uuid.New()),
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		CardID:        option.Some(valueobjects.CardIDFromUUID(uuid.New())),
		Installments:  option.Some(s.installmentCount(installments)),
		OccurredAt:    occurredAt,
	}
}

func (s *TransactionWorkflowSuite) TestDecideCreate_Income_NoCardArtifacts() {
	cmd := s.simpleCreateCmd("income", "pix", 1000, s.now)
	txID := uuid.New()
	eventID := uuid.New()

	decision := s.sut.DecideCreate(cmd, option.None[valueobjects.CardBillingSnapshot](), valueobjects.CategoryWriteEvidence{}, txID, eventID, s.createItemIDs(cmd), s.now)

	s.Nil(decision.Items)
	s.Nil(decision.InvoiceDeltas)
	s.False(decision.Transaction.BillingSnapshot().IsPresent())
	s.Equal(txID, decision.Transaction.ID())

	evt, ok := decision.Event.(entities.TransactionCreated)
	s.Require().True(ok)
	s.Equal(eventID, evt.EventID)
	s.Equal(int64(1000), evt.AmountCents)
	s.Require().Len(evt.RefMonthsAffected, 1)
	s.Equal("2024-03", evt.RefMonthsAffected[0].String())
	s.Nil(evt.Installments)
}

func (s *TransactionWorkflowSuite) TestDecideCreate_OutcomeSimple_NoCardArtifacts() {
	cmd := s.simpleCreateCmd("outcome", "debit_card", 2500, s.now)

	decision := s.sut.DecideCreate(cmd, option.None[valueobjects.CardBillingSnapshot](), valueobjects.CategoryWriteEvidence{}, uuid.New(), uuid.New(), s.createItemIDs(cmd), s.now)

	s.Nil(decision.Items)
	s.Nil(decision.InvoiceDeltas)
	evt, ok := decision.Event.(entities.TransactionCreated)
	s.Require().True(ok)
	s.Equal(int64(2500), evt.AmountCents)
	s.Nil(evt.Installments)
}

func (s *TransactionWorkflowSuite) TestDecideCreate_CreditCardSingleInstallment() {
	purchasedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	cmd := s.cardCreateCmd(1000, 1, purchasedAt)
	snap := s.snapshot(15, 25)
	txID := uuid.New()

	decision := s.sut.DecideCreate(cmd, option.Some(snap), valueobjects.CategoryWriteEvidence{}, txID, uuid.New(), s.createItemIDs(cmd), s.now)

	s.Require().Len(decision.Items, 1)
	s.Equal(int64(1000), decision.Items[0].Amount().Cents())
	s.Equal(txID, decision.Items[0].PurchaseID())
	s.True(decision.Transaction.BillingSnapshot().IsPresent())
	s.True(decision.Transaction.CardID().IsPresent())

	var deltaSum int64
	for _, d := range decision.InvoiceDeltas {
		deltaSum += d
	}
	s.Equal(int64(1000), deltaSum)

	evt, ok := decision.Event.(entities.TransactionCreated)
	s.Require().True(ok)
	s.Require().Len(evt.Installments, 1)
	s.Require().Len(evt.RefMonthsAffected, 1)
	s.Equal("2024-01", evt.RefMonthsAffected[0].String())
}

func (s *TransactionWorkflowSuite) TestDecideCreate_CreditCard12x_SumEqualsTotalAndRefMonthsDistributed() {
	purchasedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	cmd := s.cardCreateCmd(10000, 12, purchasedAt)
	snap := s.snapshot(15, 25)

	decision := s.sut.DecideCreate(cmd, option.Some(snap), valueobjects.CategoryWriteEvidence{}, uuid.New(), uuid.New(), s.createItemIDs(cmd), s.now)

	s.Require().Len(decision.Items, 12)

	var itemSum int64
	seen := map[string]struct{}{}
	for _, it := range decision.Items {
		itemSum += it.Amount().Cents()
		seen[it.RefMonth().String()] = struct{}{}
	}
	s.Equal(int64(10000), itemSum, "soma das parcelas deve ser exatamente igual ao total")
	s.Len(seen, 12, "12 competências distintas")

	var deltaSum int64
	for _, d := range decision.InvoiceDeltas {
		deltaSum += d
	}
	s.Equal(int64(10000), deltaSum)

	evt, ok := decision.Event.(entities.TransactionCreated)
	s.Require().True(ok)
	s.Require().Len(evt.Installments, 12)
	var evtSum int64
	for _, inst := range evt.Installments {
		evtSum += inst.AmountCents
	}
	s.Equal(int64(10000), evtSum)
	s.Len(evt.RefMonthsAffected, 12)
}

func (s *TransactionWorkflowSuite) TestDecideUpdate_NonCard_SameRefMonth() {
	cmd := s.simpleCreateCmd("income", "pix", 1000, s.now)
	txID := uuid.New()
	create := s.sut.DecideCreate(cmd, option.None[valueobjects.CardBillingSnapshot](), valueobjects.CategoryWriteEvidence{}, txID, uuid.New(), s.createItemIDs(cmd), s.now)

	desc, _ := valueobjects.NewDescription("Updated")
	updateCmd := commands.UpdateTransaction{
		TransactionID: txID,
		UserID:        cmd.UserID,
		Direction:     cmd.Direction,
		PaymentMethod: cmd.PaymentMethod,
		Amount:        s.money(2000),
		Description:   desc,
		CategoryID:    valueobjects.CategoryIDFromUUID(uuid.New()),
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		OccurredAt:    time.Date(2024, 3, 20, 12, 0, 0, 0, time.UTC),
		Version:       1,
	}

	decision := s.sut.DecideUpdate(create.Transaction, nil, updateCmd, valueobjects.CategoryWriteEvidence{}, uuid.New(), s.updateItemIDs(updateCmd), s.now.Add(time.Hour))

	s.Nil(decision.Items)
	s.Nil(decision.InvoiceDeltas)
	evt, ok := decision.Event.(entities.TransactionUpdated)
	s.Require().True(ok)
	s.Require().Len(evt.RefMonthsAffected, 1)
	s.Equal("2024-03", evt.RefMonthsAffected[0].String())
}

func (s *TransactionWorkflowSuite) TestDecideUpdate_NonCard_ChangedRefMonth() {
	cmd := s.simpleCreateCmd("income", "pix", 1000, s.now)
	txID := uuid.New()
	create := s.sut.DecideCreate(cmd, option.None[valueobjects.CardBillingSnapshot](), valueobjects.CategoryWriteEvidence{}, txID, uuid.New(), s.createItemIDs(cmd), s.now)

	desc, _ := valueobjects.NewDescription("Updated")
	updateCmd := commands.UpdateTransaction{
		TransactionID: txID,
		UserID:        cmd.UserID,
		Direction:     cmd.Direction,
		PaymentMethod: cmd.PaymentMethod,
		Amount:        s.money(2000),
		Description:   desc,
		CategoryID:    valueobjects.CategoryIDFromUUID(uuid.New()),
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		OccurredAt:    time.Date(2024, 4, 5, 12, 0, 0, 0, time.UTC),
		Version:       1,
	}

	decision := s.sut.DecideUpdate(create.Transaction, nil, updateCmd, valueobjects.CategoryWriteEvidence{}, uuid.New(), s.updateItemIDs(updateCmd), s.now.Add(time.Hour))

	evt, ok := decision.Event.(entities.TransactionUpdated)
	s.Require().True(ok)
	s.Require().Len(evt.RefMonthsAffected, 2)
	refs := []string{evt.RefMonthsAffected[0].String(), evt.RefMonthsAffected[1].String()}
	s.Contains(refs, "2024-03")
	s.Contains(refs, "2024-04")
}

func (s *TransactionWorkflowSuite) TestDecideUpdate_CreditCard_RecomputesDeltas_12to3() {
	purchasedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	snap := s.snapshot(15, 25)
	createCmd := s.cardCreateCmd(12000, 12, purchasedAt)
	txID := uuid.New()
	create := s.sut.DecideCreate(createCmd, option.Some(snap), valueobjects.CategoryWriteEvidence{}, txID, uuid.New(), s.createItemIDs(createCmd), s.now)

	desc, _ := valueobjects.NewDescription("Updated card")
	updateCmd := commands.UpdateTransaction{
		TransactionID: txID,
		UserID:        createCmd.UserID,
		Direction:     valueobjects.DirectionOutcome,
		PaymentMethod: createCmd.PaymentMethod,
		Amount:        s.money(3000),
		Description:   desc,
		CategoryID:    valueobjects.CategoryIDFromUUID(uuid.New()),
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		CardID:        createCmd.CardID,
		Installments:  option.Some(s.installmentCount(3)),
		OccurredAt:    purchasedAt,
		Version:       1,
	}

	decision := s.sut.DecideUpdate(create.Transaction, create.Items, updateCmd, valueobjects.CategoryWriteEvidence{}, uuid.New(), s.updateItemIDs(updateCmd), s.now.Add(time.Hour))

	s.Require().Len(decision.Items, 3)

	var newSum int64
	for _, it := range decision.Items {
		newSum += it.Amount().Cents()
	}
	s.Equal(int64(3000), newSum)

	var deltaSum int64
	negatives := 0
	for _, d := range decision.InvoiceDeltas {
		deltaSum += d
		if d < 0 {
			negatives++
		}
	}
	s.Equal(int64(3000-12000), deltaSum, "delta líquido = novo total - total antigo")
	s.Equal(9, negatives, "9 competências removidas devem ter delta negativo")

	evt, ok := decision.Event.(entities.TransactionUpdated)
	s.Require().True(ok)
	s.Len(evt.RefMonthsAffected, 12)
}

func (s *TransactionWorkflowSuite) TestDecideDelete_CreditCard_ReversesDeltas() {
	purchasedAt := time.Date(2024, 11, 10, 0, 0, 0, 0, time.UTC)
	snap := s.snapshot(15, 25)
	createCmd := s.cardCreateCmd(3000, 3, purchasedAt)
	create := s.sut.DecideCreate(createCmd, option.Some(snap), valueobjects.CategoryWriteEvidence{}, uuid.New(), uuid.New(), s.createItemIDs(createCmd), s.now)

	decision, err := s.sut.DecideDelete(create.Transaction, create.Items, uuid.New(), s.now.Add(time.Hour))
	s.Require().NoError(err)

	s.Nil(decision.Items)
	s.Require().NotNil(decision.InvoiceDeltas)

	var deltaSum int64
	for _, d := range decision.InvoiceDeltas {
		s.Less(d, int64(0), "cada delta deve ser negativo (reversão)")
		deltaSum += d
	}
	s.Equal(int64(-3000), deltaSum, "reversão total sem saldo residual")

	evt, ok := decision.Event.(entities.TransactionDeleted)
	s.Require().True(ok)
	s.Len(evt.RefMonthsAffected, 3)
	s.NotNil(decision.Transaction.DeletedAt())
}

func (s *TransactionWorkflowSuite) TestDecideDelete_NonCard_SingleRefMonth() {
	cmd := s.simpleCreateCmd("income", "pix", 1000, s.now)
	create := s.sut.DecideCreate(cmd, option.None[valueobjects.CardBillingSnapshot](), valueobjects.CategoryWriteEvidence{}, uuid.New(), uuid.New(), s.createItemIDs(cmd), s.now)

	decision, err := s.sut.DecideDelete(create.Transaction, nil, uuid.New(), s.now.Add(time.Hour))
	s.Require().NoError(err)

	s.Nil(decision.InvoiceDeltas)
	evt, ok := decision.Event.(entities.TransactionDeleted)
	s.Require().True(ok)
	s.Require().Len(evt.RefMonthsAffected, 1)
	s.Equal("2024-03", evt.RefMonthsAffected[0].String())
}

func (s *TransactionWorkflowSuite) TestDecideDelete_AlreadyDeleted_ReturnsError() {
	cmd := s.simpleCreateCmd("income", "pix", 1000, s.now)
	create := s.sut.DecideCreate(cmd, option.None[valueobjects.CardBillingSnapshot](), valueobjects.CategoryWriteEvidence{}, uuid.New(), uuid.New(), s.createItemIDs(cmd), s.now)

	first, err := s.sut.DecideDelete(create.Transaction, nil, uuid.New(), s.now.Add(time.Hour))
	s.Require().NoError(err)

	_, err = s.sut.DecideDelete(first.Transaction, nil, uuid.New(), s.now.Add(2*time.Hour))
	s.Require().Error(err)
	s.ErrorIs(err, entities.ErrTransactionAlreadyDeleted)
}

func (s *TransactionWorkflowSuite) TestDecideCreate_Deterministic() {
	purchasedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	snap := s.snapshot(15, 25)
	cmd := s.cardCreateCmd(10000, 12, purchasedAt)
	txID := uuid.New()
	eventID := uuid.New()
	itemIDs := s.createItemIDs(cmd)

	first := s.sut.DecideCreate(cmd, option.Some(snap), valueobjects.CategoryWriteEvidence{}, txID, eventID, itemIDs, s.now)
	second := s.sut.DecideCreate(cmd, option.Some(snap), valueobjects.CategoryWriteEvidence{}, txID, eventID, itemIDs, s.now)

	s.Require().Len(first.Items, len(second.Items))
	for i := range first.Items {
		s.Equal(first.Items[i].ID(), second.Items[i].ID(), "item IDs devem ser determinísticos dado o mesmo input")
		s.Equal(first.Items[i].Amount().Cents(), second.Items[i].Amount().Cents())
		s.Equal(first.Items[i].RefMonth().String(), second.Items[i].RefMonth().String())
	}

	firstEvt := first.Event.(entities.TransactionCreated)
	secondEvt := second.Event.(entities.TransactionCreated)
	s.Require().Len(firstEvt.RefMonthsAffected, len(secondEvt.RefMonthsAffected))
	for i := range firstEvt.RefMonthsAffected {
		s.Equal(firstEvt.RefMonthsAffected[i].String(), secondEvt.RefMonthsAffected[i].String())
	}
}
