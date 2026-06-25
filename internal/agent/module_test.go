package agent

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

type recordingInterpreter struct {
	resp        interfaces.LLMResponse
	lastRequest interfaces.LLMRequest
	calls       int
}

func (r *recordingInterpreter) Interpret(_ context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	r.calls++
	r.lastRequest = req
	return r.resp, nil
}

type stubSessionRepo struct{}

func (s *stubSessionRepo) Create(_ context.Context, _ interfaces.AgentSessionRecord) error {
	return nil
}

func (s *stubSessionRepo) GetByUserAndChannel(_ context.Context, _ uuid.UUID, _ string) (interfaces.AgentSessionRecord, error) {
	return interfaces.AgentSessionRecord{}, interfaces.ErrAgentSessionNotFound
}

func (s *stubSessionRepo) Update(_ context.Context, _ interfaces.AgentSessionRecord) error {
	return nil
}

func (s *stubSessionRepo) Upsert(_ context.Context, _ interfaces.AgentSessionRecord) error {
	return nil
}

func (s *stubSessionRepo) DeleteExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

type stubWorkingMemoryRepo struct{}

func (s *stubWorkingMemoryRepo) Get(_ context.Context, _ uuid.UUID) (entities.WorkingMemory, bool, error) {
	return entities.WorkingMemory{}, false, nil
}

func (s *stubWorkingMemoryRepo) Upsert(_ context.Context, _ entities.WorkingMemory) error {
	return nil
}

type stubObservationRepo struct{}

func (s *stubObservationRepo) Insert(_ context.Context, _ entities.Observation) error {
	return nil
}

func (s *stubObservationRepo) ListRecent(_ context.Context, _ uuid.UUID, _ string, _ int) ([]entities.Observation, error) {
	return nil, nil
}

func (s *stubObservationRepo) DeleteExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (s *stubObservationRepo) DeleteOldestBeyondLimit(_ context.Context, _ uuid.UUID, _ string, _ int) error {
	return nil
}

func TestRebuildConversationalReplyUsesConversationalInterpreterAndTokens(t *testing.T) {
	parseInterpreter := &recordingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("parse")}}
	convInterpreter := &recordingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("reply")}}
	wiring := &agentModuleWiring{
		cfg: &configs.Config{
			AgentConfig: configs.AgentConfig{
				ProseMaxTokens: 111,
			},
		},
		o11y:        fake.NewProvider(),
		sessionRepo: &stubSessionRepo{},
		wmRepo:      &stubWorkingMemoryRepo{},
		obsRepo:     &stubObservationRepo{},
	}
	runtime := &llmRuntime{
		Interpreter:     parseInterpreter,
		ConvInterpreter: convInterpreter,
		ConvMaxTokens:   222,
	}

	uc, err := rebuildConversationalReply(wiring, runtime, nil)
	require.NoError(t, err)

	out, err := uc.Execute(context.Background(), usecases.ComposeConversationalInput{
		UserID:  uuid.New(),
		Channel: "whatsapp",
		Text:    "oi",
	})
	require.NoError(t, err)
	require.Equal(t, "reply", out.Reply)
	require.Equal(t, 1, convInterpreter.calls)
	require.Equal(t, 0, parseInterpreter.calls)
	require.Equal(t, 222, convInterpreter.lastRequest.MaxTokens)
}

func TestRebuildConversationalReplyFallsBackToProseTokens(t *testing.T) {
	convInterpreter := &recordingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("reply")}}
	wiring := &agentModuleWiring{
		cfg: &configs.Config{
			AgentConfig: configs.AgentConfig{
				ProseMaxTokens: 333,
			},
		},
		o11y:        fake.NewProvider(),
		sessionRepo: &stubSessionRepo{},
		wmRepo:      &stubWorkingMemoryRepo{},
		obsRepo:     &stubObservationRepo{},
	}
	runtime := &llmRuntime{
		Interpreter:     convInterpreter,
		ConvInterpreter: convInterpreter,
	}

	uc, err := rebuildConversationalReply(wiring, runtime, nil)
	require.NoError(t, err)

	_, err = uc.Execute(context.Background(), usecases.ComposeConversationalInput{
		UserID:  uuid.New(),
		Channel: "whatsapp",
		Text:    "oi",
	})
	require.NoError(t, err)
	require.Equal(t, 333, convInterpreter.lastRequest.MaxTokens)
}

func TestResolveParseMaxTokensUsesParseWhenPositive(t *testing.T) {
	require.Equal(t, 900, resolveParseMaxTokens(900, 768))
}

func TestResolveParseMaxTokensFallsBackToGlobalWhenZero(t *testing.T) {
	require.Equal(t, 768, resolveParseMaxTokens(0, 768))
}
