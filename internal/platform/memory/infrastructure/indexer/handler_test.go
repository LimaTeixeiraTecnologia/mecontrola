package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	memorymocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type mockEmbedder struct {
	mock.Mock
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	args := m.Called(ctx, texts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([][]float32), args.Error(1)
}

type mockEvent struct {
	eventType string
	payload   any
}

func (e *mockEvent) GetEventType() string { return e.eventType }
func (e *mockEvent) GetPayload() any      { return e.payload }

type EmbeddingIndexHandlerSuite struct {
	suite.Suite
	ctx         context.Context
	embedderMck *mockEmbedder
	recallMck   *memorymocks.SemanticRecall
}

func TestEmbeddingIndexHandlerSuite(t *testing.T) {
	suite.Run(t, new(EmbeddingIndexHandlerSuite))
}

func (s *EmbeddingIndexHandlerSuite) SetupTest() {
	s.ctx = context.Background()
	s.embedderMck = &mockEmbedder{}
	s.recallMck = memorymocks.NewSemanticRecall(s.T())
}

func (s *EmbeddingIndexHandlerSuite) TestHandle_WithOutboxEnvelope() {
	msgPK := uuid.New()
	payload := memory.IndexMessagePayload{
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		MessageID:  msgPK,
		Content:    "hello world",
		Model:      "text-embedding-3-small",
	}
	raw, _ := json.Marshal(payload)

	env := outbox.Envelope{
		ID:        uuid.NewString(),
		EventType: memory.EventTypeEmbeddingIndex,
		Payload:   json.RawMessage(raw),
	}

	embedding := []float32{0.1, 0.2, 0.3}

	s.embedderMck.On("Embed", s.ctx, []string{payload.Content}).Return([][]float32{embedding}, nil).Once()
	s.recallMck.EXPECT().Index(s.ctx, payload.ResourceID, payload.ThreadID, msgPK, payload.Content, payload.Model, embedding).Return(nil).Once()

	sut := NewEmbeddingIndexHandler(s.embedderMck, s.recallMck, fake.NewProvider())
	err := sut.Handle(s.ctx, &mockEvent{eventType: memory.EventTypeEmbeddingIndex, payload: env})

	s.NoError(err)
	s.embedderMck.AssertExpectations(s.T())
}

func (s *EmbeddingIndexHandlerSuite) TestHandle_EmptyContent_Skips() {
	payload := memory.IndexMessagePayload{
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		MessageID:  uuid.New(),
		Content:    "",
		Model:      "text-embedding-3-small",
	}
	raw, _ := json.Marshal(payload)
	env := outbox.Envelope{Payload: json.RawMessage(raw)}

	sut := NewEmbeddingIndexHandler(s.embedderMck, s.recallMck, fake.NewProvider())
	err := sut.Handle(s.ctx, &mockEvent{payload: env})

	s.NoError(err)
	s.embedderMck.AssertNotCalled(s.T(), "Embed")
}

func (s *EmbeddingIndexHandlerSuite) TestHandle_EmbedError_ReturnsError() {
	payload := memory.IndexMessagePayload{
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		MessageID:  uuid.New(),
		Content:    "hello",
		Model:      "text-embedding-3-small",
	}
	raw, _ := json.Marshal(payload)
	env := outbox.Envelope{Payload: json.RawMessage(raw)}

	s.embedderMck.On("Embed", s.ctx, []string{payload.Content}).Return(nil, errors.New("api error")).Once()

	sut := NewEmbeddingIndexHandler(s.embedderMck, s.recallMck, fake.NewProvider())
	err := sut.Handle(s.ctx, &mockEvent{payload: env})

	s.Error(err)
	s.embedderMck.AssertExpectations(s.T())
}

func (s *EmbeddingIndexHandlerSuite) TestHandle_IndexError_ReturnsError() {
	msgPK := uuid.New()
	payload := memory.IndexMessagePayload{
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		MessageID:  msgPK,
		Content:    "hello",
		Model:      "text-embedding-3-small",
	}
	raw, _ := json.Marshal(payload)
	env := outbox.Envelope{Payload: json.RawMessage(raw)}

	embedding := []float32{0.1, 0.2}

	s.embedderMck.On("Embed", s.ctx, []string{payload.Content}).Return([][]float32{embedding}, nil).Once()
	s.recallMck.EXPECT().Index(s.ctx, payload.ResourceID, payload.ThreadID, msgPK, payload.Content, payload.Model, embedding).Return(errors.New("db error")).Once()

	sut := NewEmbeddingIndexHandler(s.embedderMck, s.recallMck, fake.NewProvider())
	err := sut.Handle(s.ctx, &mockEvent{payload: env})

	s.Error(err)
}

func (s *EmbeddingIndexHandlerSuite) TestHandle_IdempotentBySourceMessageID() {
	msgPK := uuid.New()
	payload := memory.IndexMessagePayload{
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		MessageID:  msgPK,
		Content:    "hello world",
		Model:      "text-embedding-3-small",
	}
	raw, _ := json.Marshal(payload)

	embedding := []float32{0.5, 0.6}

	s.embedderMck.On("Embed", s.ctx, []string{payload.Content}).Return([][]float32{embedding}, nil).Twice()
	s.recallMck.EXPECT().Index(s.ctx, payload.ResourceID, payload.ThreadID, msgPK, payload.Content, payload.Model, embedding).Return(nil).Twice()

	sut := NewEmbeddingIndexHandler(s.embedderMck, s.recallMck, fake.NewProvider())

	env := outbox.Envelope{Payload: json.RawMessage(raw)}
	evt := &mockEvent{payload: env}

	err1 := sut.Handle(s.ctx, evt)
	err2 := sut.Handle(s.ctx, evt)

	s.NoError(err1)
	s.NoError(err2)
}
