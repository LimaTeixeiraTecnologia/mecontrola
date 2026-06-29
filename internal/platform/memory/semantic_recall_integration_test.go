//go:build integration

package memory_test

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"testing"
	"time"

	"hash/fnv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/indexer"
	memorypostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

const embeddingModel = "text-embedding-3-small"

type stubEmbedder struct{}

func (stubEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = embedVector(t)
	}
	return out, nil
}

func embedVector(text string) []float32 {
	v := make([]float32, 1536)
	h := fnv.New64a()
	for d := 0; d < 1536; d++ {
		h.Reset()
		_, _ = io.WriteString(h, text)
		_, _ = io.WriteString(h, "#")
		_, _ = io.WriteString(h, strconv.Itoa(d))
		v[d] = float32(h.Sum64()%1000) / 1000.0
	}
	return v
}

type capturingPublisher struct {
	captured []memory.IndexMessagePayload
}

func (c *capturingPublisher) PublishIndex(ctx context.Context, p memory.IndexMessagePayload) error {
	c.captured = append(c.captured, p)
	return nil
}

type indexEvent struct {
	payload any
}

func (e *indexEvent) GetEventType() string { return memory.EventTypeEmbeddingIndex }
func (e *indexEvent) GetPayload() any      { return e.payload }

type SemanticRecallIntegrationSuite struct {
	suite.Suite
	ctx        context.Context
	db         *sqlx.DB
	obs        observability.Observability
	threadRepo memory.ThreadGateway
	msgRepo    memory.MessageStore
	embedRepo  memory.SemanticRecall
}

func TestSemanticRecallIntegrationSuite(t *testing.T) {
	suite.Run(t, new(SemanticRecallIntegrationSuite))
}

func (s *SemanticRecallIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.db, _ = testcontainer.Postgres(s.T())
	s.threadRepo = memorypostgres.NewThreadRepository(s.db, s.obs)
	s.msgRepo = memorypostgres.NewMessageRepository(s.db, s.obs)
	s.embedRepo = memorypostgres.NewEmbeddingRepository(s.db, s.obs)
}

func (s *SemanticRecallIntegrationSuite) buildEvent(p memory.IndexMessagePayload) events.Event {
	raw, err := json.Marshal(p)
	s.Require().NoError(err)
	env := outbox.Envelope{
		ID:        uuid.NewString(),
		EventType: memory.EventTypeEmbeddingIndex,
		Payload:   json.RawMessage(raw),
	}
	return &indexEvent{payload: env}
}

func (s *SemanticRecallIntegrationSuite) TestAsyncChainScopedRecall() {
	resourceA := "res-A-" + uuid.NewString()
	resourceB := "res-B-" + uuid.NewString()
	const content = "rainy weather in Lisbon"

	threadA, err := s.threadRepo.GetOrCreate(s.ctx, resourceA, "thr-1")
	s.Require().NoError(err)

	pub := &capturingPublisher{}
	store := memory.NewPublishingMessageStore(s.msgRepo, pub, embeddingModel, s.obs)

	msg := memory.Message{
		ID:         uuid.New(),
		ThreadPK:   threadA.ID,
		ResourceID: resourceA,
		Role:       memory.RoleUser,
		Content:    content,
		CreatedAt:  time.Now().UTC(),
	}
	s.Require().NoError(store.Append(s.ctx, threadA.ID, msg))

	s.Require().Len(pub.captured, 1)
	s.Equal(resourceA, pub.captured[0].ResourceID)
	s.Equal(content, pub.captured[0].Content)
	s.Equal(msg.ID, pub.captured[0].MessagePK)
	s.Equal(embeddingModel, pub.captured[0].Model)

	handler := indexer.NewEmbeddingIndexHandler(stubEmbedder{}, s.embedRepo, s.obs)
	s.Require().NoError(handler.Handle(s.ctx, s.buildEvent(pub.captured[0])))

	hits, err := s.embedRepo.Recall(s.ctx, resourceA, content, embedVector(content), 5)
	s.Require().NoError(err)
	s.Require().NotEmpty(hits)
	s.Equal(content, hits[0].Content)
	s.Equal(resourceA, hits[0].ResourceID)

	leaked, err := s.embedRepo.Recall(s.ctx, resourceB, content, embedVector(content), 5)
	s.Require().NoError(err)
	s.Empty(leaked)
}

func (s *SemanticRecallIntegrationSuite) TestReplayDoesNotDuplicateEmbedding() {
	resource := "res-replay-" + uuid.NewString()
	const content = "snow forecast in Oslo"

	thread, err := s.threadRepo.GetOrCreate(s.ctx, resource, "thr-replay")
	s.Require().NoError(err)

	msgPK := uuid.New()
	payload := memory.IndexMessagePayload{
		ResourceID: resource,
		ThreadID:   thread.ID.String(),
		MessagePK:  msgPK,
		Content:    content,
		Model:      embeddingModel,
	}

	handler := indexer.NewEmbeddingIndexHandler(stubEmbedder{}, s.embedRepo, s.obs)
	event := s.buildEvent(payload)

	s.Require().NoError(handler.Handle(s.ctx, event))
	s.Require().NoError(handler.Handle(s.ctx, event))

	var count int
	err = s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.platform_embeddings WHERE source_message_pk=$1`, msgPK,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}
