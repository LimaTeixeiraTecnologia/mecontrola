package usecases

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/media"
)

type mockAudioMediaClient struct {
	mock.Mock
}

func (m *mockAudioMediaClient) Resolve(ctx context.Context, mediaID string) (media.MediaDescriptor, error) {
	args := m.Called(ctx, mediaID)
	return args.Get(0).(media.MediaDescriptor), args.Error(1)
}

func (m *mockAudioMediaClient) Download(ctx context.Context, url string, maxBytes int64) (media.DownloadedMedia, error) {
	args := m.Called(ctx, url, maxBytes)
	return args.Get(0).(media.DownloadedMedia), args.Error(1)
}

type mockTranscriber struct {
	mock.Mock
}

func (m *mockTranscriber) Transcribe(ctx context.Context, req llm.TranscriptionRequest) (llm.TranscriptionResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(llm.TranscriptionResponse), args.Error(1)
}

type mockAudioAuditRepository struct {
	mock.Mock
}

func (m *mockAudioAuditRepository) InsertProcessing(ctx context.Context, record AudioAuditRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *mockAudioAuditRepository) FinalizeTerminal(ctx context.Context, record AudioAuditRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *mockAudioAuditRepository) FindByWAMID(ctx context.Context, wamid string) (AudioAuditRecord, error) {
	args := m.Called(ctx, wamid)
	return args.Get(0).(AudioAuditRecord), args.Error(1)
}

func (m *mockAudioAuditRepository) UpdateOutcome(ctx context.Context, wamid string, outcome AudioOutcome, reason AudioReason, completedAt time.Time) error {
	args := m.Called(ctx, wamid, outcome, reason, completedAt)
	return args.Error(0)
}

func buildOggOpusFixture(preSkip uint16, finalGranule int64) []byte {
	headPayload := make([]byte, 19)
	copy(headPayload[0:8], []byte("OpusHead"))
	headPayload[8] = 1
	headPayload[9] = 2
	binary.LittleEndian.PutUint16(headPayload[10:12], preSkip)
	binary.LittleEndian.PutUint32(headPayload[12:16], 48000)
	binary.LittleEndian.PutUint16(headPayload[16:18], 0)
	headPayload[18] = 0

	idPage := buildOggPage(0x02, 0, headPayload)
	dataPage := buildOggPage(0x04, finalGranule, []byte("opus-frame-bytes"))

	fixture := make([]byte, 0, len(idPage)+len(dataPage))
	fixture = append(fixture, idPage...)
	fixture = append(fixture, dataPage...)
	return fixture
}

func buildOggPage(headerType byte, granule int64, payload []byte) []byte {
	segments := lacingValues(len(payload))

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

func lacingValues(payloadLen int) []byte {
	values := make([]byte, 0, payloadLen/255+1)
	remaining := payloadLen
	for remaining >= 255 {
		values = append(values, 255)
		remaining -= 255
	}
	values = append(values, byte(remaining))
	return values
}

const testWAMID = "wamid-audio-001"
const testUserID = "6f9619ff-8b86-d011-b42d-00cf4fc964ff"
const testPeer = "+5511999999999"
const testMediaID = "media-001"
const testMimeType = "audio/ogg"
const testSHA256 = "sha256-abc"

func validAudioInboundInput() AudioInboundInput {
	return AudioInboundInput{
		WAMID:    testWAMID,
		UserID:   testUserID,
		Peer:     testPeer,
		MediaID:  testMediaID,
		MimeType: testMimeType,
		SHA256:   testSHA256,
	}
}

func baseCfg() ProcessAudioInboundConfig {
	return ProcessAudioInboundConfig{
		STTModel:        "openai/whisper-large-v3",
		MaxDuration:     60 * time.Second,
		MaxBytes:        2_000_000,
		MaxCostMicrousd: 2000,
		MinConfidence:   0.80,
	}
}

type ProcessAudioInboundSuite struct {
	suite.Suite
	ctx context.Context
}

func TestProcessAudioInboundSuite(t *testing.T) {
	suite.Run(t, new(ProcessAudioInboundSuite))
}

func (s *ProcessAudioInboundSuite) SetupTest() {
	s.ctx = context.Background()
}

type audioInboundDeps struct {
	mediaMock       *mockAudioMediaClient
	transcriberMock *mockTranscriber
	auditMock       *mockAudioAuditRepository
}

type audioInboundScenario struct {
	name         string
	input        AudioInboundInput
	cfg          ProcessAudioInboundConfig
	dependencies audioInboundDeps
	expect       func(result AudioInboundResult, err error)
}

func (s *ProcessAudioInboundSuite) runAudioScenarios(scenarios []audioInboundScenario) {
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewProcessAudioInbound(
				scenario.dependencies.mediaMock,
				scenario.dependencies.transcriberMock,
				scenario.dependencies.auditMock,
				scenario.cfg,
				fake.NewProvider(),
			)
			result, err := uc.Execute(s.ctx, scenario.input)
			scenario.expect(result, err)
			scenario.dependencies.mediaMock.AssertExpectations(s.T())
			scenario.dependencies.transcriberMock.AssertExpectations(s.T())
			scenario.dependencies.auditMock.AssertExpectations(s.T())
		})
	}
}

func (s *ProcessAudioInboundSuite) TestExecuteTranscriptionOutcomes() {
	shortFixture := buildOggOpusFixture(0, 48000)

	scenarios := []audioInboundScenario{
		{
			name:  "deve aprovar audio pt-br confiavel e persistir auditoria",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock: func() *mockAudioMediaClient {
					m := &mockAudioMediaClient{}
					m.On("Resolve", mock.Anything, testMediaID).
						Return(media.MediaDescriptor{URL: "https://meta/media/1", MimeType: testMimeType, SHA256: "hash-1", Size: int64(len(shortFixture))}, nil).Once()
					m.On("Download", mock.Anything, "https://meta/media/1", int64(2_000_000)).
						Return(media.DownloadedMedia{Bytes: shortFixture, MimeType: testMimeType, SHA256: "hash-1", Size: int64(len(shortFixture))}, nil).Once()
					return m
				}(),
				transcriberMock: func() *mockTranscriber {
					m := &mockTranscriber{}
					m.On("Transcribe", mock.Anything, mock.AnythingOfType("llm.TranscriptionRequest")).
						Return(llm.TranscriptionResponse{Text: "gastei 50 reais no mercado", Language: "pt", Model: "openai/whisper-large-v3"}, nil).Once()
					return m
				}(),
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.WAMID == testWAMID &&
							r.Outcome == AudioOutcomeProcessing &&
							r.Reason == AudioReasonProcessing
					})).Return(nil).Once()
					m.On("FinalizeTerminal", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.Outcome == AudioOutcomeApproved && r.Reason == AudioReasonApproved && r.Transcription != nil
					})).Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeApproved, result.Outcome)
				s.Equal("gastei 50 reais no mercado", result.CanonicalText)
				s.Empty(result.ReplyText)
			},
		},
		{
			name:  "deve marcar incerto quando stt retorna texto truncado sem chamar handle inbound",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock: func() *mockAudioMediaClient {
					m := &mockAudioMediaClient{}
					m.On("Resolve", mock.Anything, testMediaID).
						Return(media.MediaDescriptor{URL: "https://meta/media/1", MimeType: testMimeType, SHA256: "hash-2", Size: int64(len(shortFixture))}, nil).Once()
					m.On("Download", mock.Anything, "https://meta/media/1", int64(2_000_000)).
						Return(media.DownloadedMedia{Bytes: shortFixture, MimeType: testMimeType, SHA256: "hash-2", Size: int64(len(shortFixture))}, nil).Once()
					return m
				}(),
				transcriberMock: func() *mockTranscriber {
					m := &mockTranscriber{}
					m.On("Transcribe", mock.Anything, mock.AnythingOfType("llm.TranscriptionRequest")).
						Return(llm.TranscriptionResponse{Text: "gastei 50 reais", Language: "pt", Truncated: true}, nil).Once()
					return m
				}(),
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.WAMID == testWAMID &&
							r.Outcome == AudioOutcomeProcessing &&
							r.Reason == AudioReasonProcessing
					})).Return(nil).Once()
					m.On("FinalizeTerminal", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.Outcome == AudioOutcomeTranscriptionUncertain && r.Reason == AudioReasonTruncated && r.Transcription == nil
					})).Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeTranscriptionUncertain, result.Outcome)
				s.Empty(result.CanonicalText)
				s.NotEmpty(result.ReplyText)
				s.False(result.Outcome.AllowsHandleInbound())
			},
		},
		{
			name:  "deve falhar tecnicamente quando stt retorna erro upstream",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock: func() *mockAudioMediaClient {
					m := &mockAudioMediaClient{}
					m.On("Resolve", mock.Anything, testMediaID).
						Return(media.MediaDescriptor{URL: "https://meta/media/1", MimeType: testMimeType, SHA256: "hash-3", Size: int64(len(shortFixture))}, nil).Once()
					m.On("Download", mock.Anything, "https://meta/media/1", int64(2_000_000)).
						Return(media.DownloadedMedia{Bytes: shortFixture, MimeType: testMimeType, SHA256: "hash-3", Size: int64(len(shortFixture))}, nil).Once()
					return m
				}(),
				transcriberMock: func() *mockTranscriber {
					m := &mockTranscriber{}
					m.On("Transcribe", mock.Anything, mock.AnythingOfType("llm.TranscriptionRequest")).
						Return(llm.TranscriptionResponse{}, llm.ErrSTTUpstream).Once()
					return m
				}(),
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.WAMID == testWAMID &&
							r.Outcome == AudioOutcomeProcessing &&
							r.Reason == AudioReasonProcessing
					})).Return(nil).Once()
					m.On("FinalizeTerminal", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.Outcome == AudioOutcomeTranscriptionFailed && r.Reason == AudioReasonSTTError
					})).Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeTranscriptionFailed, result.Outcome)
				s.False(result.Outcome.AllowsHandleInbound())
			},
		},
		{
			name:  "deve falhar tecnicamente sem persistir quando decision input do caminho de erro stt e invalido",
			input: validAudioInboundInput(),
			cfg: func() ProcessAudioInboundConfig {
				cfg := baseCfg()
				cfg.STTModel = ""
				return cfg
			}(),
			dependencies: audioInboundDeps{
				mediaMock: func() *mockAudioMediaClient {
					m := &mockAudioMediaClient{}
					m.On("Resolve", mock.Anything, testMediaID).
						Return(media.MediaDescriptor{URL: "https://meta/media/1", MimeType: testMimeType, SHA256: "hash-3b", Size: int64(len(shortFixture))}, nil).Once()
					m.On("Download", mock.Anything, "https://meta/media/1", int64(2_000_000)).
						Return(media.DownloadedMedia{Bytes: shortFixture, MimeType: testMimeType, SHA256: "hash-3b", Size: int64(len(shortFixture))}, nil).Once()
					return m
				}(),
				transcriberMock: func() *mockTranscriber {
					m := &mockTranscriber{}
					m.On("Transcribe", mock.Anything, mock.AnythingOfType("llm.TranscriptionRequest")).
						Return(llm.TranscriptionResponse{}, llm.ErrSTTUpstream).Once()
					return m
				}(),
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.WAMID == testWAMID &&
							r.Outcome == AudioOutcomeProcessing &&
							r.Reason == AudioReasonProcessing
					})).Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.Error(err)
				s.Contains(err.Error(), "decision input invalido")
				s.Equal(AudioInboundResult{}, result)
			},
		},
	}

	s.runAudioScenarios(scenarios)
}

func (s *ProcessAudioInboundSuite) TestExecuteCostRejection() {
	shortFixture := buildOggOpusFixture(0, 48000)

	scenarios := []audioInboundScenario{
		{
			name:  "deve rejeitar quando custo pre-stt excede o teto configurado",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock: func() *mockAudioMediaClient {
					m := &mockAudioMediaClient{}
					m.On("Resolve", mock.Anything, testMediaID).
						Return(media.MediaDescriptor{URL: "https://meta/media/1", MimeType: testMimeType, SHA256: "hash-4", Size: int64(len(shortFixture))}, nil).Once()
					m.On("Download", mock.Anything, "https://meta/media/1", int64(2_000_000)).
						Return(media.DownloadedMedia{Bytes: shortFixture, MimeType: testMimeType, SHA256: "hash-4", Size: int64(len(shortFixture))}, nil).Once()
					return m
				}(),
				transcriberMock: func() *mockTranscriber {
					m := &mockTranscriber{}
					m.On("Transcribe", mock.Anything, mock.AnythingOfType("llm.TranscriptionRequest")).
						Return(llm.TranscriptionResponse{}, llm.ErrSTTCostPreflightExceeded).Once()
					return m
				}(),
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.WAMID == testWAMID &&
							r.Outcome == AudioOutcomeProcessing &&
							r.Reason == AudioReasonProcessing
					})).Return(nil).Once()
					m.On("FinalizeTerminal", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.Outcome == AudioOutcomeRejected && r.Reason == AudioReasonCostExceeded
					})).Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeRejected, result.Outcome)
				s.Equal(AudioReasonCostExceeded, result.Reason)
			},
		},
	}

	s.runAudioScenarios(scenarios)
}

func (s *ProcessAudioInboundSuite) TestExecuteRejections() {
	longFixture := buildOggOpusFixture(312, 312+96000)

	scenarios := []audioInboundScenario{
		{
			name:  "deve rejeitar quando download excede o tamanho maximo sem chamar stt",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock: func() *mockAudioMediaClient {
					m := &mockAudioMediaClient{}
					m.On("Resolve", mock.Anything, testMediaID).
						Return(media.MediaDescriptor{URL: "https://meta/media/1", MimeType: testMimeType, SHA256: "hash-5", Size: int64(3_000_000)}, nil).Once()
					m.On("Download", mock.Anything, "https://meta/media/1", int64(2_000_000)).
						Return(media.DownloadedMedia{}, media.ErrDownloadTooLarge).Once()
					return m
				}(),
				transcriberMock: &mockTranscriber{},
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.WAMID == testWAMID &&
							r.Outcome == AudioOutcomeProcessing &&
							r.Reason == AudioReasonProcessing
					})).Return(nil).Once()
					m.On("FinalizeTerminal", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.Outcome == AudioOutcomeRejected && r.Reason == AudioReasonSizeExceeded
					})).Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeRejected, result.Outcome)
				s.Equal(AudioReasonSizeExceeded, result.Reason)
			},
		},
		{
			name:  "deve rejeitar quando media api falha ao resolver o media id",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock: func() *mockAudioMediaClient {
					m := &mockAudioMediaClient{}
					m.On("Resolve", mock.Anything, testMediaID).
						Return(media.MediaDescriptor{}, errors.New("meta indisponivel")).Once()
					return m
				}(),
				transcriberMock: &mockTranscriber{},
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.WAMID == testWAMID &&
							r.Outcome == AudioOutcomeProcessing &&
							r.Reason == AudioReasonProcessing
					})).Return(nil).Once()
					m.On("FinalizeTerminal", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.Outcome == AudioOutcomeRejected && r.Reason == AudioReasonMediaUnavailable
					})).Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeRejected, result.Outcome)
				s.Equal(AudioReasonMediaUnavailable, result.Reason)
			},
		},
		{
			name:  "deve rejeitar quando duracao excede o maximo configurado sem chamar stt",
			input: validAudioInboundInput(),
			cfg:   ProcessAudioInboundConfig{STTModel: "openai/whisper-large-v3", MaxDuration: 1 * time.Second, MaxBytes: 2_000_000, MaxCostMicrousd: 2000, MinConfidence: 0.80},
			dependencies: audioInboundDeps{
				mediaMock: func() *mockAudioMediaClient {
					m := &mockAudioMediaClient{}
					m.On("Resolve", mock.Anything, testMediaID).
						Return(media.MediaDescriptor{URL: "https://meta/media/1", MimeType: testMimeType, SHA256: "hash-6", Size: int64(len(longFixture))}, nil).Once()
					m.On("Download", mock.Anything, "https://meta/media/1", int64(2_000_000)).
						Return(media.DownloadedMedia{Bytes: longFixture, MimeType: testMimeType, SHA256: "hash-6", Size: int64(len(longFixture))}, nil).Once()
					return m
				}(),
				transcriberMock: &mockTranscriber{},
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.WAMID == testWAMID &&
							r.Outcome == AudioOutcomeProcessing &&
							r.Reason == AudioReasonProcessing
					})).Return(nil).Once()
					m.On("FinalizeTerminal", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.Outcome == AudioOutcomeRejected && r.Reason == AudioReasonDurationExceeded
					})).Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeRejected, result.Outcome)
				s.Equal(AudioReasonDurationExceeded, result.Reason)
			},
		},
		{
			name:  "deve rejeitar quando duracao nao pode ser determinada sem chamar stt",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock: func() *mockAudioMediaClient {
					m := &mockAudioMediaClient{}
					m.On("Resolve", mock.Anything, testMediaID).
						Return(media.MediaDescriptor{URL: "https://meta/media/1", MimeType: testMimeType, SHA256: "hash-7", Size: int64(20)}, nil).Once()
					m.On("Download", mock.Anything, "https://meta/media/1", int64(2_000_000)).
						Return(media.DownloadedMedia{Bytes: []byte("nao e um audio valido"), MimeType: testMimeType, SHA256: "hash-7", Size: 20}, nil).Once()
					return m
				}(),
				transcriberMock: &mockTranscriber{},
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.WAMID == testWAMID &&
							r.Outcome == AudioOutcomeProcessing &&
							r.Reason == AudioReasonProcessing
					})).Return(nil).Once()
					m.On("FinalizeTerminal", mock.Anything, mock.MatchedBy(func(r AudioAuditRecord) bool {
						return r.Outcome == AudioOutcomeRejected && r.Reason == AudioReasonDurationUnavailable
					})).Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeRejected, result.Outcome)
				s.Equal(AudioReasonDurationUnavailable, result.Reason)
			},
		},
	}

	s.runAudioScenarios(scenarios)
}

func (s *ProcessAudioInboundSuite) TestExecuteAuditAndIdempotency() {
	scenarios := []audioInboundScenario{
		{
			name:  "deve retornar resultado idempotente quando wamid ja foi processado sem chamar media ou stt",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock:       &mockAudioMediaClient{},
				transcriberMock: &mockTranscriber{},
				auditMock: func() *mockAudioAuditRepository {
					transcription := "gastei 30 no uber"
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{
							WAMID: testWAMID, Outcome: AudioOutcomeApproved, Reason: AudioReasonApproved,
							Transcription: &transcription,
						}, nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeApproved, result.Outcome)
				s.Equal("gastei 30 no uber", result.CanonicalText)
			},
		},
		{
			name:  "deve finalizar wamid interrompido sem reprocessar media nem stt",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock:       &mockAudioMediaClient{},
				transcriberMock: &mockTranscriber{},
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{
							WAMID: testWAMID, Outcome: AudioOutcomeProcessing, Reason: AudioReasonProcessing,
						}, nil).Once()
					m.On("UpdateOutcome", mock.Anything, testWAMID,
						AudioOutcomeTranscriptionFailed, AudioReasonInterrupted, mock.AnythingOfType("time.Time")).
						Return(nil).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.NoError(err)
				s.Equal(AudioOutcomeTranscriptionFailed, result.Outcome)
				s.Equal(AudioReasonInterrupted, result.Reason)
				s.NotEmpty(result.ReplyText, "usuario deve receber resposta em vez de silencio apos execucao interrompida")
				s.False(result.Outcome.AllowsHandleInbound())
			},
		},
		{
			name:  "nao deve baixar midia quando abrir auditoria falhar",
			input: validAudioInboundInput(),
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock:       &mockAudioMediaClient{},
				transcriberMock: &mockTranscriber{},
				auditMock: func() *mockAudioAuditRepository {
					m := &mockAudioAuditRepository{}
					m.On("FindByWAMID", mock.Anything, testWAMID).
						Return(AudioAuditRecord{}, ErrAudioAuditNotFound).Once()
					m.On("InsertProcessing", mock.Anything, mock.AnythingOfType("usecases.AudioAuditRecord")).
						Return(errors.New("postgres indisponivel")).Once()
					return m
				}(),
			},
			expect: func(result AudioInboundResult, err error) {
				s.Error(err, "ADR-002: sem linha de auditoria aberta, nada pode ser baixado nem transcrito")
				s.Equal(AudioOutcomeProcessing.String(), "processing")
			},
		},
		{
			name:  "deve retornar erro tipado quando input estiver invalido",
			input: AudioInboundInput{},
			cfg:   baseCfg(),
			dependencies: audioInboundDeps{
				mediaMock:       &mockAudioMediaClient{},
				transcriberMock: &mockTranscriber{},
				auditMock:       &mockAudioAuditRepository{},
			},
			expect: func(result AudioInboundResult, err error) {
				s.Error(err)
				s.True(errors.Is(err, ErrAudioInboundInvalidInput))
			},
		},
	}

	s.runAudioScenarios(scenarios)
}

func (s *ProcessAudioInboundSuite) TestMarkDispatched() {
	auditMock := &mockAudioAuditRepository{}
	auditMock.On("UpdateOutcome", mock.Anything, testWAMID, AudioOutcomeDispatched, AudioReasonApproved, mock.AnythingOfType("time.Time")).
		Return(nil).Once()

	uc := NewProcessAudioInbound(&mockAudioMediaClient{}, &mockTranscriber{}, auditMock, baseCfg(), fake.NewProvider())
	err := uc.MarkDispatched(s.ctx, testWAMID)
	s.NoError(err)
	auditMock.AssertExpectations(s.T())
}

func (s *ProcessAudioInboundSuite) TestMarkDispatchedRequiresWAMID() {
	uc := NewProcessAudioInbound(&mockAudioMediaClient{}, &mockTranscriber{}, &mockAudioAuditRepository{}, baseCfg(), fake.NewProvider())
	err := uc.MarkDispatched(s.ctx, "")
	s.Error(err)
	s.True(errors.Is(err, ErrAudioInboundInvalidInput))
}
