package workflows

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	ifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func (s *PendingEntryWorkflowSuite) resumePayloadWithID(text, messageID string) []byte {
	b, _ := json.Marshal(map[string]string{"resumeText": text, "incomingMessageId": messageID})
	return b
}

func (s *PendingEntryWorkflowSuite) TestResume_Confirmation_DirectWrite_EmptyResource_FailsTypedKeepsActive_RF10() {
	state := s.newState(AwaitingSlotConfirmation)
	k := s.key("thr-confirm-direct-empty")

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: uuid.Nil}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))

	s.Error(err)
	s.ErrorIs(err, ErrWriteAcceptedWithoutResource)
	s.Equal(workflow.RunStatusFailed, result.Status)
	s.Equal(PendingStatusActive, result.State.Status)
	s.NotEqual(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Confirmation_IdempotentWrite_EmptyResource_FailsTypedKeepsActive_RF10() {
	state := s.newState(AwaitingSlotConfirmation)
	k := s.key("thr-confirm-idem-empty")
	def := BuildPendingEntryWorkflow(s.ledger, nil, nil, &fakeIdempotentWriter{
		results: []struct {
			id      uuid.UUID
			outcome agent.ToolOutcome
			err     error
		}{
			{id: uuid.Nil, outcome: agent.ToolOutcomeRouted, err: nil},
		},
	})

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: uuid.Nil}, nil).
		Maybe()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resumePayload("sim"))

	s.Error(err)
	s.ErrorIs(err, ErrWriteAcceptedWithoutResource)
	s.Equal(workflow.RunStatusFailed, result.Status)
	s.Equal(PendingStatusActive, result.State.Status)
	s.NotEqual(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Confirmation_Accept_SetsProcessedMessageID_ADR002() {
	state := s.newState(AwaitingSlotConfirmation)
	k := s.key("thr-confirm-accept-processed")

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayloadWithID("sim", "wamid-confirm-002"))

	s.NoError(err)
	s.Equal(PendingStatusCompleted, result.State.Status)
	s.Equal("wamid-confirm-002", result.State.ProcessedMessageID)
}

func (s *PendingEntryWorkflowSuite) TestShouldExpireAfterFailedWrite() {
	now := time.Now().UTC()
	base := s.newState(AwaitingSlotConfirmation)

	notExpired := base
	notExpired.SuspendedAt = now.Add(-1 * time.Minute)
	s.False(ShouldExpireAfterFailedWrite(notExpired, now))

	expired := base
	expired.SuspendedAt = now.Add(-31 * time.Minute)
	s.True(ShouldExpireAfterFailedWrite(expired, now))

	completed := expired
	completed.Status = PendingStatusCompleted
	s.False(ShouldExpireAfterFailedWrite(completed, now))
}
