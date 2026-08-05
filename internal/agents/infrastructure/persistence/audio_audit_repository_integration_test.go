//go:build integration

package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type AudioAuditIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	repo usecases.AudioAuditRepository
}

func TestAudioAuditIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AudioAuditIntegrationSuite))
}

func (s *AudioAuditIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	s.repo = persistence.NewAudioAuditRepository(db, fake.NewProvider())
}

func int64Ptr(v int64) *int64 { return &v }

func newAudioAuditRecord(wamid string, outcome usecases.AudioOutcome, reason usecases.AudioReason) usecases.AudioAuditRecord {
	return usecases.AudioAuditRecord{
		WAMID:       wamid,
		UserID:      uuid.New(),
		Peer:        "5511999998888",
		MediaID:     "media-" + uuid.NewString(),
		MediaSHA256: "sha256-" + uuid.NewString(),
		MimeType:    "audio/ogg",
		SizeBytes:   int64Ptr(1024),
		Outcome:     outcome,
		Reason:      reason,
		CreatedAt:   time.Now().UTC(),
	}
}

func (s *AudioAuditIntegrationSuite) seedTerminal(record usecases.AudioAuditRecord) error {
	opening := record
	opening.Outcome = usecases.AudioOutcomeProcessing
	opening.Reason = usecases.AudioReasonProcessing
	if err := s.repo.InsertProcessing(s.ctx, opening); err != nil {
		return err
	}
	return s.repo.FinalizeTerminal(s.ctx, record)
}

func (s *AudioAuditIntegrationSuite) TestInsertProcessingBeforeDownloadThenFinalize_ADR002() {
	wamid := "wamid-audio-adr002-" + uuid.NewString()
	opening := newAudioAuditRecord(wamid, usecases.AudioOutcomeProcessing, usecases.AudioReasonProcessing)

	s.Require().NoError(s.repo.InsertProcessing(s.ctx, opening))

	inflight, err := s.repo.FindByWAMID(s.ctx, wamid)
	s.Require().NoError(err, "ADR-002: a linha deve existir antes do download/STT")
	s.Equal(usecases.AudioOutcomeProcessing, inflight.Outcome)
	s.Nil(inflight.CompletedAt)

	terminal := newAudioAuditRecord(wamid, usecases.AudioOutcomeApproved, usecases.AudioReasonApproved)
	completedAt := time.Now().UTC()
	terminal.CompletedAt = &completedAt
	s.Require().NoError(s.repo.FinalizeTerminal(s.ctx, terminal))

	found, findErr := s.repo.FindByWAMID(s.ctx, wamid)
	s.Require().NoError(findErr)
	s.Equal(usecases.AudioOutcomeApproved, found.Outcome)
	s.NotNil(found.CompletedAt)
}

func (s *AudioAuditIntegrationSuite) TestFinalizeTerminalIsNotIdempotentlyReappliable_RF22() {
	wamid := "wamid-audio-refinalize-" + uuid.NewString()
	record := newAudioAuditRecord(wamid, usecases.AudioOutcomeTranscriptionFailed, usecases.AudioReasonSTTError)
	s.Require().NoError(s.seedTerminal(record))

	second := newAudioAuditRecord(wamid, usecases.AudioOutcomeApproved, usecases.AudioReasonApproved)
	err := s.repo.FinalizeTerminal(s.ctx, second)
	s.Require().ErrorIs(err, usecases.ErrAudioAuditNotFound,
		"linha ja terminal nao pode ser refinalizada; so transita a partir de processing")

	found, findErr := s.repo.FindByWAMID(s.ctx, wamid)
	s.Require().NoError(findErr)
	s.Equal(usecases.AudioOutcomeTranscriptionFailed, found.Outcome)
}

func (s *AudioAuditIntegrationSuite) TestInterruptedProcessingCanBeFinalizedWithoutReprocessing_ADR002() {
	wamid := "wamid-audio-interrupted-" + uuid.NewString()
	opening := newAudioAuditRecord(wamid, usecases.AudioOutcomeProcessing, usecases.AudioReasonProcessing)
	s.Require().NoError(s.repo.InsertProcessing(s.ctx, opening))

	s.Require().NoError(s.repo.UpdateOutcome(s.ctx, wamid,
		usecases.AudioOutcomeTranscriptionFailed, usecases.AudioReasonInterrupted, time.Now().UTC()))

	found, err := s.repo.FindByWAMID(s.ctx, wamid)
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeTranscriptionFailed, found.Outcome)
	s.Equal(usecases.AudioReasonInterrupted, found.Reason)

	retry := newAudioAuditRecord(wamid, usecases.AudioOutcomeApproved, usecases.AudioReasonApproved)
	s.Require().ErrorIs(s.repo.InsertProcessing(s.ctx, retry), usecases.ErrAudioAuditAlreadyExists,
		"wamid interrompido e terminal: nao pode reabrir processamento")
}

func (s *AudioAuditIntegrationSuite) TestInsertTerminalAndFindByWAMID() {
	wamid := "wamid-audio-" + uuid.NewString()
	record := newAudioAuditRecord(wamid, usecases.AudioOutcomeApproved, usecases.AudioReasonApproved)

	s.Require().NoError(s.seedTerminal(record))

	found, err := s.repo.FindByWAMID(s.ctx, wamid)
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeApproved, found.Outcome)
	s.Equal(usecases.AudioReasonApproved, found.Reason)
	s.Equal(record.MediaSHA256, found.MediaSHA256)
	s.Equal(record.UserID, found.UserID)
}

func (s *AudioAuditIntegrationSuite) TestInsertTerminalDuplicateWAMIDReturnsAlreadyExists_RF22() {
	wamid := "wamid-audio-dup-" + uuid.NewString()
	first := newAudioAuditRecord(wamid, usecases.AudioOutcomeTranscriptionFailed, usecases.AudioReasonSTTError)

	s.Require().NoError(s.seedTerminal(first))

	second := newAudioAuditRecord(wamid, usecases.AudioOutcomeApproved, usecases.AudioReasonApproved)
	err := s.repo.InsertProcessing(s.ctx, second)
	s.Require().ErrorIs(err, usecases.ErrAudioAuditAlreadyExists)

	found, findErr := s.repo.FindByWAMID(s.ctx, wamid)
	s.Require().NoError(findErr)
	s.Equal(usecases.AudioOutcomeTranscriptionFailed, found.Outcome, "outcome terminal original nao pode ser sobrescrito por segunda tentativa")
	s.Equal(usecases.AudioReasonSTTError, found.Reason)
}

func (s *AudioAuditIntegrationSuite) TestFindByWAMIDNotFound() {
	_, err := s.repo.FindByWAMID(s.ctx, "wamid-does-not-exist-"+uuid.NewString())
	s.Require().ErrorIs(err, usecases.ErrAudioAuditNotFound)
}

func (s *AudioAuditIntegrationSuite) TestUpdateOutcomeTransitionsApprovedToDispatched() {
	wamid := "wamid-audio-dispatch-" + uuid.NewString()
	record := newAudioAuditRecord(wamid, usecases.AudioOutcomeApproved, usecases.AudioReasonApproved)
	s.Require().NoError(s.seedTerminal(record))

	completedAt := time.Now().UTC()
	s.Require().NoError(s.repo.UpdateOutcome(s.ctx, wamid, usecases.AudioOutcomeDispatched, usecases.AudioReasonApproved, completedAt))

	found, err := s.repo.FindByWAMID(s.ctx, wamid)
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeDispatched, found.Outcome)
	s.Require().NotNil(found.CompletedAt)
}

func (s *AudioAuditIntegrationSuite) TestUpdateOutcomeNotFound() {
	err := s.repo.UpdateOutcome(s.ctx, "wamid-missing-"+uuid.NewString(), usecases.AudioOutcomeDispatched, usecases.AudioReasonApproved, time.Now().UTC())
	s.Require().ErrorIs(err, usecases.ErrAudioAuditNotFound)
}

func (s *AudioAuditIntegrationSuite) TestInsertTerminalRejectedNeverReprocessed_RF22() {
	wamid := "wamid-audio-rejected-" + uuid.NewString()
	record := newAudioAuditRecord(wamid, usecases.AudioOutcomeRejected, usecases.AudioReasonEmptyText)
	s.Require().NoError(s.seedTerminal(record))

	retry := newAudioAuditRecord(wamid, usecases.AudioOutcomeApproved, usecases.AudioReasonApproved)
	err := s.repo.InsertProcessing(s.ctx, retry)
	s.Require().ErrorIs(err, usecases.ErrAudioAuditAlreadyExists)

	found, findErr := s.repo.FindByWAMID(s.ctx, wamid)
	s.Require().NoError(findErr)
	s.Equal(usecases.AudioOutcomeRejected, found.Outcome, "falha terminal nao pode ser reprocessada automaticamente pelo mesmo wamid")
}

func (s *AudioAuditIntegrationSuite) TestInsertTerminalAcceptsPreSTTRejectionReasons() {
	preSTTReasons := []usecases.AudioReason{
		usecases.AudioReasonInvalidPayload,
		usecases.AudioReasonMediaUnavailable,
		usecases.AudioReasonSizeExceeded,
		usecases.AudioReasonDurationUnavailable,
		usecases.AudioReasonDurationExceeded,
		usecases.AudioReasonCostExceeded,
	}
	for _, reason := range preSTTReasons {
		wamid := "wamid-audio-prestt-" + uuid.NewString()
		record := newAudioAuditRecord(wamid, usecases.AudioOutcomeRejected, reason)
		s.Require().NoError(s.seedTerminal(record), "reason %s deve ser aceita pelo CHECK constraint da migration 000016", reason)

		found, err := s.repo.FindByWAMID(s.ctx, wamid)
		s.Require().NoError(err)
		s.Equal(reason, found.Reason)
	}
}
