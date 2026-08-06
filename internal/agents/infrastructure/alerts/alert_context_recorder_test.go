package alerts_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/alerts"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type stubThreadGateway struct {
	thread memory.Thread
	err    error
}

func (s *stubThreadGateway) GetOrCreate(_ context.Context, resourceID, threadID string) (memory.Thread, error) {
	if s.err != nil {
		return memory.Thread{}, s.err
	}
	s.thread.ResourceID = resourceID
	s.thread.ThreadID = threadID
	return s.thread, nil
}

type stubMessageStore struct {
	appended []memory.Message
	err      error
}

func (s *stubMessageStore) Append(_ context.Context, _ uuid.UUID, m memory.Message) error {
	if s.err != nil {
		return s.err
	}
	s.appended = append(s.appended, m)
	return nil
}

func (s *stubMessageStore) Recent(_ context.Context, _ uuid.UUID, _ int) ([]memory.Message, error) {
	return s.appended, nil
}

type stubWorkingMemory struct {
	metadata  map[string]any
	upserts   []map[string]any
	upsertErr error
	getErr    error
}

func (s *stubWorkingMemory) Get(_ context.Context, _ string) (string, error) { return "", nil }

func (s *stubWorkingMemory) Upsert(_ context.Context, _, _ string) error { return nil }

func (s *stubWorkingMemory) UpsertMetadata(_ context.Context, _ string, metadata map[string]any) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.upserts = append(s.upserts, metadata)
	if s.metadata == nil {
		s.metadata = map[string]any{}
	}
	for k, v := range metadata {
		s.metadata[k] = v
	}
	return nil
}

func (s *stubWorkingMemory) GetMetadata(_ context.Context, _ string) (map[string]any, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.metadata, nil
}

type AlertContextRecorderSuite struct {
	suite.Suite
	ctx  context.Context
	obs  observability.Observability
	now  time.Time
	ttl  time.Duration
	pk   uuid.UUID
	wm   *stubWorkingMemory
	msgs *stubMessageStore
	thr  *stubThreadGateway
}

func TestAlertContextRecorderSuite(t *testing.T) {
	suite.Run(t, new(AlertContextRecorderSuite))
}

func (s *AlertContextRecorderSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.now = time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	s.ttl = 24 * time.Hour
	s.pk = uuid.New()
	s.wm = &stubWorkingMemory{}
	s.msgs = &stubMessageStore{}
	s.thr = &stubThreadGateway{thread: memory.Thread{ID: s.pk}}
}

func (s *AlertContextRecorderSuite) recorder() *alerts.AlertContextRecorder {
	return alerts.NewAlertContextRecorder(s.thr, s.msgs, s.wm, s.ttl, s.obs)
}

func (s *AlertContextRecorderSuite) TestRecordAppendsMessageAndMetadata() {
	err := s.recorder().Record(s.ctx, "user-1", "+5511999990000", "category_threshold_80", "detalhar a categoria por subcategoria", s.now)
	s.Require().NoError(err)

	s.Require().Len(s.msgs.appended, 1)
	message := s.msgs.appended[0]
	s.Equal(memory.RoleAssistant, message.Role)
	s.Equal(s.pk, message.PlatformThreadID)
	s.Equal("user-1", message.ResourceID)
	s.True(strings.Contains(message.Content, "category_threshold_80"))
	s.True(strings.Contains(message.Content, "detalhar a categoria por subcategoria"))

	s.Require().Len(s.wm.upserts, 1)
	entry, ok := s.wm.upserts[0][alerts.MetadataKeyLastProactiveAlert].(map[string]any)
	s.Require().True(ok)
	s.Equal("category_threshold_80", entry["kind"])
	s.Equal(s.now.Format(time.RFC3339), entry["sent_at"])
}

func (s *AlertContextRecorderSuite) TestRecordRejectsMissingIdentifiers() {
	scenarios := []struct {
		name       string
		resourceID string
		threadID   string
		kind       string
	}{
		{name: "sem resource", resourceID: "", threadID: "+55", kind: "category_threshold_80"},
		{name: "sem thread", resourceID: "user-1", threadID: "", kind: "category_threshold_80"},
		{name: "sem kind", resourceID: "user-1", threadID: "+55", kind: ""},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			err := s.recorder().Record(s.ctx, scenario.resourceID, scenario.threadID, scenario.kind, "topico", s.now)
			s.Require().Error(err)
			s.Empty(s.msgs.appended)
		})
	}
}

func (s *AlertContextRecorderSuite) TestRecordPropagatesThreadError() {
	s.thr.err = errors.New("thread down")
	err := s.recorder().Record(s.ctx, "user-1", "+55", "category_threshold_80", "topico", s.now)
	s.Require().Error(err)
	s.Empty(s.msgs.appended)
}

func (s *AlertContextRecorderSuite) TestPurgeExpiredKeepsValidContext() {
	s.wm.metadata = map[string]any{
		alerts.MetadataKeyLastProactiveAlert: map[string]any{
			"kind":    "category_threshold_80",
			"sent_at": s.now.Add(-1 * time.Hour).Format(time.RFC3339),
		},
	}

	err := s.recorder().PurgeExpired(s.ctx, "user-1", s.now)
	s.Require().NoError(err)
	s.Empty(s.wm.upserts)
}

func (s *AlertContextRecorderSuite) TestPurgeExpiredClearsStaleContext() {
	s.wm.metadata = map[string]any{
		alerts.MetadataKeyLastProactiveAlert: map[string]any{
			"kind":    "category_threshold_80",
			"sent_at": s.now.Add(-25 * time.Hour).Format(time.RFC3339),
		},
	}

	err := s.recorder().PurgeExpired(s.ctx, "user-1", s.now)
	s.Require().NoError(err)
	s.Require().Len(s.wm.upserts, 1)
	value, present := s.wm.upserts[0][alerts.MetadataKeyLastProactiveAlert]
	s.True(present)
	s.Nil(value)
}

func (s *AlertContextRecorderSuite) TestPurgeExpiredIgnoresAbsentOrMalformedContext() {
	scenarios := []struct {
		name     string
		metadata map[string]any
	}{
		{name: "sem metadata", metadata: nil},
		{name: "chave ausente", metadata: map[string]any{"outra": "coisa"}},
		{name: "chave nula", metadata: map[string]any{alerts.MetadataKeyLastProactiveAlert: nil}},
		{name: "formato invalido", metadata: map[string]any{alerts.MetadataKeyLastProactiveAlert: "texto"}},
		{name: "sent_at invalido", metadata: map[string]any{
			alerts.MetadataKeyLastProactiveAlert: map[string]any{"sent_at": "ontem"},
		}},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.wm = &stubWorkingMemory{metadata: scenario.metadata}
			err := s.recorder().PurgeExpired(s.ctx, "user-1", s.now)
			s.Require().NoError(err)
			s.Empty(s.wm.upserts)
		})
	}
}

func (s *AlertContextRecorderSuite) TestIsAlertContextValid() {
	scenarios := []struct {
		name   string
		sentAt time.Time
		ttl    time.Duration
		expect bool
	}{
		{name: "dentro do ttl", sentAt: s.now.Add(-1 * time.Hour), ttl: 24 * time.Hour, expect: true},
		{name: "exatamente no limite", sentAt: s.now.Add(-24 * time.Hour), ttl: 24 * time.Hour, expect: true},
		{name: "expirado", sentAt: s.now.Add(-25 * time.Hour), ttl: 24 * time.Hour, expect: false},
		{name: "sent_at zero", sentAt: time.Time{}, ttl: 24 * time.Hour, expect: false},
		{name: "ttl zero", sentAt: s.now, ttl: 0, expect: false},
		{name: "sent_at no futuro", sentAt: s.now.Add(time.Hour), ttl: 24 * time.Hour, expect: false},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, alerts.IsAlertContextValid(scenario.sentAt, s.now, scenario.ttl))
		})
	}
}
