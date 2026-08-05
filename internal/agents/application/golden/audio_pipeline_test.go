package golden

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/reconciliation"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/media"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type simulatedTranscribeFunc func(ctx context.Context, req llm.TranscriptionRequest) (llm.TranscriptionResponse, error)

type SimulatedTranscriber struct {
	Calls int
	fn    simulatedTranscribeFunc
}

func NewSimulatedTranscriber(fn simulatedTranscribeFunc) *SimulatedTranscriber {
	return &SimulatedTranscriber{fn: fn}
}

func (t *SimulatedTranscriber) Transcribe(ctx context.Context, req llm.TranscriptionRequest) (llm.TranscriptionResponse, error) {
	t.Calls++
	return t.fn(ctx, req)
}

type simulatedMediaClient struct {
	Calls          int
	mimeType       string
	bytes          []byte
	downloadErr    error
	resolveErr     error
	corruptedSHA25 bool
}

func newSimulatedMediaClient(mimeType string, payload []byte) *simulatedMediaClient {
	return &simulatedMediaClient{mimeType: mimeType, bytes: payload}
}

func (m *simulatedMediaClient) Resolve(_ context.Context, mediaID string) (media.MediaDescriptor, error) {
	m.Calls++
	if m.resolveErr != nil {
		return media.MediaDescriptor{}, m.resolveErr
	}
	sum := sha256.Sum256(m.bytes)
	descriptorSHA := hex.EncodeToString(sum[:])
	if m.corruptedSHA25 {
		descriptorSHA = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	return media.MediaDescriptor{
		URL:      "https://media.local/" + mediaID,
		MimeType: m.mimeType,
		SHA256:   descriptorSHA,
		Size:     int64(len(m.bytes)),
	}, nil
}

func (m *simulatedMediaClient) Download(_ context.Context, _ string, _ int64) (media.DownloadedMedia, error) {
	if m.downloadErr != nil {
		return media.DownloadedMedia{}, m.downloadErr
	}
	sum := sha256.Sum256(m.bytes)
	return media.DownloadedMedia{
		Bytes:    m.bytes,
		MimeType: m.mimeType,
		SHA256:   hex.EncodeToString(sum[:]),
		Size:     int64(len(m.bytes)),
	}, nil
}

type inMemoryAudioAuditRepository struct {
	records map[string]usecases.AudioAuditRecord
}

func newInMemoryAudioAuditRepository() *inMemoryAudioAuditRepository {
	return &inMemoryAudioAuditRepository{records: map[string]usecases.AudioAuditRecord{}}
}

func (r *inMemoryAudioAuditRepository) InsertProcessing(_ context.Context, record usecases.AudioAuditRecord) error {
	if _, exists := r.records[record.WAMID]; exists {
		return usecases.ErrAudioAuditAlreadyExists
	}
	r.records[record.WAMID] = record
	return nil
}

func (r *inMemoryAudioAuditRepository) FinalizeTerminal(_ context.Context, record usecases.AudioAuditRecord) error {
	existing, ok := r.records[record.WAMID]
	if !ok || existing.Outcome != usecases.AudioOutcomeProcessing {
		return usecases.ErrAudioAuditNotFound
	}
	record.CreatedAt = existing.CreatedAt
	r.records[record.WAMID] = record
	return nil
}

func (r *inMemoryAudioAuditRepository) FindByWAMID(_ context.Context, wamid string) (usecases.AudioAuditRecord, error) {
	record, ok := r.records[wamid]
	if !ok {
		return usecases.AudioAuditRecord{}, usecases.ErrAudioAuditNotFound
	}
	return record, nil
}

func (r *inMemoryAudioAuditRepository) UpdateOutcome(_ context.Context, wamid string, outcome usecases.AudioOutcome, reason usecases.AudioReason, completedAt time.Time) error {
	record, ok := r.records[wamid]
	if !ok {
		return usecases.ErrAudioAuditNotFound
	}
	record.Outcome = outcome
	record.Reason = reason
	record.CompletedAt = &completedAt
	r.records[wamid] = record
	return nil
}

func buildAudioOggOpusFixture(preSkip uint16, finalGranule int64) []byte {
	headPayload := make([]byte, 19)
	copy(headPayload[0:8], []byte("OpusHead"))
	headPayload[8] = 1
	headPayload[9] = 2
	binary.LittleEndian.PutUint16(headPayload[10:12], preSkip)
	binary.LittleEndian.PutUint32(headPayload[12:16], 48000)
	binary.LittleEndian.PutUint16(headPayload[16:18], 0)
	headPayload[18] = 0

	idPage := buildAudioOggPage(0x02, 0, headPayload)
	dataPage := buildAudioOggPage(0x04, finalGranule, []byte("opus-frame-bytes"))

	fixture := make([]byte, 0, len(idPage)+len(dataPage))
	fixture = append(fixture, idPage...)
	fixture = append(fixture, dataPage...)
	return fixture
}

func buildAudioOggPage(headerType byte, granule int64, payload []byte) []byte {
	segments := audioLacingValues(len(payload))

	page := make([]byte, 0, 27+len(segments)+len(payload))
	page = append(page, []byte("OggS")...)
	page = append(page, 0)
	page = append(page, headerType)

	granuleBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(granuleBytes, uint64(granule))
	page = append(page, granuleBytes...)

	page = append(page, 0, 0, 0, 0)
	page = append(page, 0, 0, 0, 0)
	page = append(page, 0, 0, 0, 0)
	page = append(page, byte(len(segments)))
	page = append(page, segments...)
	page = append(page, payload...)
	return page
}

func audioLacingValues(payloadLen int) []byte {
	values := make([]byte, 0, payloadLen/255+1)
	remaining := payloadLen
	for remaining >= 255 {
		values = append(values, 255)
		remaining -= 255
	}
	values = append(values, byte(remaining))
	return values
}

func validAudioOggFixture() []byte {
	return buildAudioOggOpusFixture(312, 312+96000)
}

func newProcessAudioInboundForTest(
	media usecases.AudioMediaClient,
	transcriber llm.Transcriber,
	audit usecases.AudioAuditRepository,
) *usecases.ProcessAudioInbound {
	return usecases.NewProcessAudioInbound(media, transcriber, audit, usecases.ProcessAudioInboundConfig{
		STTModel:        "openai/whisper-large-v3",
		MaxDuration:     60 * time.Second,
		MaxBytes:        2_000_000,
		MaxCostMicrousd: 2000,
		MinConfidence:   0.80,
		UncertainReply:  "não consegui entender esse áudio direito, pode tentar de novo? 🎙️",
		RejectedReply:   "não consegui processar esse áudio, pode reenviar?",
	}, fake.NewProvider())
}

type AudioNegativePipelineSuite struct {
	suite.Suite
	ctx context.Context
}

func TestAudioNegativePipelineSuite(t *testing.T) {
	suite.Run(t, new(AudioNegativePipelineSuite))
}

func (s *AudioNegativePipelineSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AudioNegativePipelineSuite) baseInput(wamid string) usecases.AudioInboundInput {
	return usecases.AudioInboundInput{
		WAMID:    wamid,
		UserID:   uuid.NewString(),
		Peer:     "peer-audio-golden",
		MediaID:  "media-audio-golden",
		MimeType: "audio/ogg",
		SHA256:   "will-be-overridden-by-fake",
		Voice:    true,
	}
}

func (s *AudioNegativePipelineSuite) TestNegativeRuidoIncoerente() {
	transcriber := NewSimulatedTranscriber(func(_ context.Context, _ llm.TranscriptionRequest) (llm.TranscriptionResponse, error) {
		return llm.TranscriptionResponse{Text: "@#$% %%% ###", Language: "pt", Confidence: floatPtr(0.95)}, nil
	})
	mediaClient := newSimulatedMediaClient("audio/ogg", validAudioOggFixture())
	audit := newInMemoryAudioAuditRepository()
	uc := newProcessAudioInboundForTest(mediaClient, transcriber, audit)

	result, err := uc.Execute(s.ctx, s.baseInput("wamid-audio-negative-ruido"))
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeTranscriptionUncertain, result.Outcome)
	s.Equal(usecases.AudioReasonIncoherent, result.Reason)

	s.False(result.Outcome.AllowsHandleInbound(),
		"ruído/incoerente nunca deve chamar HandleInbound (fronteira real de dispatch provada em whatsapp_inbound_consumer_test.go:TestHandleAudio_UncertainNeverCallsHandleInboundOrTools)")
}

func (s *AudioNegativePipelineSuite) TestNegativeFalaCortada() {
	transcriber := NewSimulatedTranscriber(func(_ context.Context, _ llm.TranscriptionRequest) (llm.TranscriptionResponse, error) {
		return llm.TranscriptionResponse{Text: "gastei 50 reais no merca", Language: "pt", Truncated: true, Confidence: floatPtr(0.95)}, nil
	})
	mediaClient := newSimulatedMediaClient("audio/ogg", validAudioOggFixture())
	audit := newInMemoryAudioAuditRepository()
	uc := newProcessAudioInboundForTest(mediaClient, transcriber, audit)

	result, err := uc.Execute(s.ctx, s.baseInput("wamid-audio-negative-cortado"))
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeTranscriptionUncertain, result.Outcome)
	s.Equal(usecases.AudioReasonTruncated, result.Reason)

	s.False(result.Outcome.AllowsHandleInbound(),
		"fala cortada nunca deve chamar HandleInbound (fronteira real de dispatch provada em whatsapp_inbound_consumer_test.go:TestHandleAudio_UncertainNeverCallsHandleInboundOrTools)")
}

func (s *AudioNegativePipelineSuite) TestNegativeIdiomaNaoPortugues() {
	transcriber := NewSimulatedTranscriber(func(_ context.Context, _ llm.TranscriptionRequest) (llm.TranscriptionResponse, error) {
		return llm.TranscriptionResponse{Text: "I spent 50 dollars at the market", Language: "en", Confidence: floatPtr(0.95)}, nil
	})
	mediaClient := newSimulatedMediaClient("audio/ogg", validAudioOggFixture())
	audit := newInMemoryAudioAuditRepository()
	uc := newProcessAudioInboundForTest(mediaClient, transcriber, audit)

	result, err := uc.Execute(s.ctx, s.baseInput("wamid-audio-negative-idioma"))
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeTranscriptionUncertain, result.Outcome)
	s.Equal(usecases.AudioReasonLanguageUnsupported, result.Reason)

	s.False(result.Outcome.AllowsHandleInbound(),
		"idioma não PT-BR nunca deve chamar HandleInbound (fronteira real de dispatch provada em whatsapp_inbound_consumer_test.go:TestHandleAudio_UncertainNeverCallsHandleInboundOrTools)")
}

func (s *AudioNegativePipelineSuite) TestNegativeFormatoInvalido() {
	transcriber := NewSimulatedTranscriber(func(_ context.Context, _ llm.TranscriptionRequest) (llm.TranscriptionResponse, error) {
		s.Fail("stt nao deve ser chamado quando o formato de audio nao e suportado")
		return llm.TranscriptionResponse{}, nil
	})
	mediaClient := newSimulatedMediaClient("audio/mpeg", []byte("conteudo-mp3-nao-suportado"))
	audit := newInMemoryAudioAuditRepository()
	uc := newProcessAudioInboundForTest(mediaClient, transcriber, audit)

	result, err := uc.Execute(s.ctx, s.baseInput("wamid-audio-negative-formato"))
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeRejected, result.Outcome)
	s.Equal(usecases.AudioReasonInvalidPayload, result.Reason,
		"mime nao suportado (audio/mpeg) deve ser rejeitado como formato invalido, distinguivel de falha de extracao de duracao na triagem do runbook")
	s.False(result.Outcome.AllowsHandleInbound())
	s.Zero(transcriber.Calls, "formato invalido nunca deve acionar o STT")

	s.False(result.Outcome.AllowsHandleInbound(),
		"formato invalido nunca deve chamar HandleInbound (fronteira real de dispatch provada em whatsapp_inbound_consumer_test.go:TestHandleAudio_UncertainNeverCallsHandleInboundOrTools)")
}

func (s *AudioNegativePipelineSuite) TestNegativeTimeoutSTT() {
	transcriber := NewSimulatedTranscriber(func(_ context.Context, _ llm.TranscriptionRequest) (llm.TranscriptionResponse, error) {
		return llm.TranscriptionResponse{}, errors.Join(llm.ErrSTTTimeout, errors.New("simulated timeout"))
	})
	mediaClient := newSimulatedMediaClient("audio/ogg", validAudioOggFixture())
	audit := newInMemoryAudioAuditRepository()
	uc := newProcessAudioInboundForTest(mediaClient, transcriber, audit)

	result, err := uc.Execute(s.ctx, s.baseInput("wamid-audio-negative-timeout"))
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeTranscriptionFailed, result.Outcome)
	s.Equal(usecases.AudioReasonSTTError, result.Reason)
	s.False(result.Outcome.AllowsHandleInbound())
	s.Equal(1, transcriber.Calls)

	s.False(result.Outcome.AllowsHandleInbound(),
		"timeout de STT nunca deve chamar HandleInbound (fronteira real de dispatch provada em whatsapp_inbound_consumer_test.go:TestHandleAudio_UncertainNeverCallsHandleInboundOrTools)")
}

func (s *AudioNegativePipelineSuite) TestNegativeDuplicado() {
	transcriber := NewSimulatedTranscriber(func(_ context.Context, _ llm.TranscriptionRequest) (llm.TranscriptionResponse, error) {
		return llm.TranscriptionResponse{}, errors.Join(llm.ErrSTTTimeout, errors.New("simulated timeout"))
	})
	mediaClient := newSimulatedMediaClient("audio/ogg", validAudioOggFixture())
	audit := newInMemoryAudioAuditRepository()
	uc := newProcessAudioInboundForTest(mediaClient, transcriber, audit)

	wamid := "wamid-audio-negative-duplicado"
	first, err := uc.Execute(s.ctx, s.baseInput(wamid))
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeTranscriptionFailed, first.Outcome)

	second, err := uc.Execute(s.ctx, s.baseInput(wamid))
	s.Require().NoError(err)
	s.Equal(first.Outcome, second.Outcome, "mensagem duplicada pelo mesmo wamid retorna o mesmo outcome terminal, sem reprocessar")
	s.Equal(first.Reason, second.Reason)

	s.Equal(1, mediaClient.Calls, "wamid duplicado nao deve tocar a media api novamente")
	s.Equal(1, transcriber.Calls, "wamid duplicado nao deve acionar o stt novamente")

	s.False(second.Outcome.AllowsHandleInbound(),
		"duplicado nunca deve chamar HandleInbound (fronteira real de dispatch provada em whatsapp_inbound_consumer_test.go:TestHandleAudio_UncertainNeverCallsHandleInboundOrTools)")
}

func (s *AudioNegativePipelineSuite) TestPositiveApprovedFlowsThroughSimulatedTranscriberWithoutOpenRouter() {
	transcriber := NewSimulatedTranscriber(func(_ context.Context, _ llm.TranscriptionRequest) (llm.TranscriptionResponse, error) {
		return llm.TranscriptionResponse{
			Text:         "gastei 50 reais no mercado hoje pagamento debito",
			Language:     "pt",
			Model:        "openai/whisper-large-v3",
			Provider:     "openrouter",
			Confidence:   floatPtr(0.97),
			CostMicrousd: int64Ptr(1200),
		}, nil
	})
	mediaClient := newSimulatedMediaClient("audio/ogg", validAudioOggFixture())
	audit := newInMemoryAudioAuditRepository()
	uc := newProcessAudioInboundForTest(mediaClient, transcriber, audit)

	result, err := uc.Execute(s.ctx, s.baseInput("wamid-audio-positive-simulado"))
	s.Require().NoError(err)
	s.Equal(usecases.AudioOutcomeApproved, result.Outcome)
	s.True(result.Outcome.AllowsHandleInbound())
	s.Equal("gastei 50 reais no mercado hoje pagamento debito", result.CanonicalText)

	s.True(result.Outcome.AllowsHandleInbound(), "audio aprovado deve permitir dispatch downstream ao HandleInbound")
}

func (s *AudioNegativePipelineSuite) TestInvariantNoFalseSuccessOnAudioOriginatedRun() {
	rows := []reconciliation.RunConsistencyRow{
		{
			RunID:             uuid.NewString(),
			CorrelationKey:    "wamid-audio-positive-simulado",
			RunStatus:         agent.RunStatusSucceeded,
			Outcome:           agent.ToolOutcomeRouted,
			LedgerWrites:      1,
			TransactionCount:  1,
			WorkflowStatus:    workflow.RunStatusSucceeded,
			WorkflowFound:     true,
			WorkflowStateSet:  true,
			WorkflowStateText: workflow.RunStatusSucceeded.String(),
		},
	}
	s.Empty(reconciliation.DecideViolations(rows), "run originado de audio com efeito durável e estados concordantes não pode gerar violação")

	falseSuccess := []reconciliation.RunConsistencyRow{
		{
			RunID:            uuid.NewString(),
			CorrelationKey:   "wamid-audio-negative-timeout",
			RunStatus:        agent.RunStatusSucceeded,
			Outcome:          agent.ToolOutcomeRouted,
			LedgerWrites:     0,
			TransactionCount: 0,
			WorkflowFound:    false,
		},
	}
	violations := reconciliation.DecideViolations(falseSuccess)
	s.Require().Len(violations, 1)
	s.Equal(reconciliation.ViolationSucceededNoEffect, violations[0].Kind, "sucesso reportado sem efeito financeiro apos audio e detectado como falso-sucesso")
}

func floatPtr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}
