package status_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status"
)

type fakeStatusRepo struct {
	calls    []status.StatusRecord
	stored   []bool
	err      error
	errAfter int
}

func (f *fakeStatusRepo) Record(_ context.Context, record status.StatusRecord) (bool, error) {
	f.calls = append(f.calls, record)
	if f.err != nil && len(f.calls) > f.errAfter {
		return false, f.err
	}
	if len(f.stored) >= len(f.calls) {
		return f.stored[len(f.calls)-1], nil
	}
	return true, nil
}

type RecordMessageStatusSuite struct {
	suite.Suite
}

func TestRecordMessageStatusSuite(t *testing.T) {
	suite.Run(t, new(RecordMessageStatusSuite))
}

func (s *RecordMessageStatusSuite) TestExecute_PersistsEachStatus() {
	repo := &fakeStatusRepo{}
	uc := status.NewRecordMessageStatus(repo, noop.NewProvider())

	statuses := []status.MessageStatus{
		{MessageID: "w1", Status: "delivered", Timestamp: "1686000000", RecipientID: "5511"},
		{MessageID: "w1", Status: "read", Timestamp: "1686000001", RecipientID: "5511"},
	}

	err := uc.Execute(context.Background(), statuses)

	s.Require().NoError(err)
	s.Require().Len(repo.calls, 2)
	s.Equal("delivered", repo.calls[0].Status)
	s.Equal("read", repo.calls[1].Status)
	s.False(repo.calls[0].StatusAt.IsZero())
}

func (s *RecordMessageStatusSuite) TestExecute_IdempotentReplayNoError() {
	repo := &fakeStatusRepo{stored: []bool{false}}
	uc := status.NewRecordMessageStatus(repo, noop.NewProvider())

	statuses := []status.MessageStatus{
		{MessageID: "w1", Status: "delivered", Timestamp: "1686000000"},
	}

	err := uc.Execute(context.Background(), statuses)

	s.Require().NoError(err)
	s.Len(repo.calls, 1)
}

func (s *RecordMessageStatusSuite) TestExecute_FailedStatusPersisted() {
	repo := &fakeStatusRepo{}
	uc := status.NewRecordMessageStatus(repo, noop.NewProvider())

	statuses := []status.MessageStatus{
		{MessageID: "w9", Status: "failed", Timestamp: "1686000000", ErrorCode: "131026", ErrorTitle: "undeliverable"},
	}

	err := uc.Execute(context.Background(), statuses)

	s.Require().NoError(err)
	s.Require().Len(repo.calls, 1)
	s.Equal("failed", repo.calls[0].Status)
	s.Equal("131026", repo.calls[0].ErrorCode)
}

func (s *RecordMessageStatusSuite) TestExecute_RepositoryErrorPropagates() {
	repo := &fakeStatusRepo{err: errors.New("pg down")}
	uc := status.NewRecordMessageStatus(repo, noop.NewProvider())

	statuses := []status.MessageStatus{
		{MessageID: "w1", Status: "sent", Timestamp: "1686000000"},
	}

	err := uc.Execute(context.Background(), statuses)

	s.Require().Error(err)
	s.Contains(err.Error(), "pg down")
}

func (s *RecordMessageStatusSuite) TestExecute_InvalidTimestampFallsBackToNow() {
	repo := &fakeStatusRepo{}
	uc := status.NewRecordMessageStatus(repo, noop.NewProvider())

	statuses := []status.MessageStatus{
		{MessageID: "w1", Status: "sent", Timestamp: "not-a-number"},
	}

	err := uc.Execute(context.Background(), statuses)

	s.Require().NoError(err)
	s.Require().Len(repo.calls, 1)
	s.False(repo.calls[0].StatusAt.IsZero())
}
