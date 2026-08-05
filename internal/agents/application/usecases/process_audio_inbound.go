package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/media"
)

var ErrAudioInboundInvalidInput = errors.New("agents.usecase.process_audio_inbound: input invalido")

const (
	defaultAudioUncertainReply        = "não consegui entender esse áudio direito, pode tentar de novo? 🎙️"
	defaultAudioRejectedReply         = "não consegui processar esse áudio, pode reenviar?"
	defaultAudioAlreadyProcessedReply = "já processei esse áudio antes, se precisar de algo mais é só chamar 🙂"

	audioSTTProvider   = "openrouter"
	audioMediaProvider = "meta"

	audioDownloadOutcomeSuccess = "success"
	audioDownloadOutcomeError   = "error"
)

type AudioMediaClient interface {
	Resolve(ctx context.Context, mediaID string) (media.MediaDescriptor, error)
	Download(ctx context.Context, url string, maxBytes int64) (media.DownloadedMedia, error)
}

type ProcessAudioInboundConfig struct {
	STTModel        string
	MaxDuration     time.Duration
	MaxBytes        int64
	MaxCostMicrousd int64
	MinConfidence   float64
	UncertainReply  string
	RejectedReply   string
}

type AudioInboundInput struct {
	WAMID    string
	UserID   string
	Peer     string
	MediaID  string
	MimeType string
	SHA256   string
	Voice    bool
}

type AudioInboundResult struct {
	Outcome       AudioOutcome
	Reason        AudioReason
	CanonicalText string
	ReplyText     string
}

type ProcessAudioInbound struct {
	media                AudioMediaClient
	transcriber          llm.Transcriber
	audit                AudioAuditRepository
	cfg                  ProcessAudioInboundConfig
	o11y                 observability.Observability
	inboundTotal         observability.Counter
	transcriptionLatency observability.Histogram
	downloadLatency      observability.Histogram
	sizeBytes            observability.Histogram
	durationSeconds      observability.Histogram
	costMicrousdTotal    observability.Counter
}

func NewProcessAudioInbound(
	mediaClient AudioMediaClient,
	transcriber llm.Transcriber,
	audit AudioAuditRepository,
	cfg ProcessAudioInboundConfig,
	o11y observability.Observability,
) *ProcessAudioInbound {
	inboundTotal := o11y.Metrics().Counter(
		"agents_audio_inbound_total",
		"Total de mensagens de audio inbound processadas por outcome/reason",
		"1",
	)
	transcriptionLatency := o11y.Metrics().HistogramWithBuckets(
		"agents_audio_transcription_latency_seconds",
		"Latencia de transcricao de audio por provider/model/outcome",
		"s",
		[]float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 20},
	)
	downloadLatency := o11y.Metrics().HistogramWithBuckets(
		"agents_audio_download_latency_seconds",
		"Latencia de download de midia de audio por provider/outcome",
		"s",
		[]float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
	)
	sizeBytes := o11y.Metrics().HistogramWithBuckets(
		"agents_audio_size_bytes",
		"Tamanho em bytes do audio inbound por mime_family/outcome",
		"By",
		[]float64{10000, 50000, 100000, 250000, 500000, 1000000, 2000000, 5000000},
	)
	durationSeconds := o11y.Metrics().HistogramWithBuckets(
		"agents_audio_duration_seconds",
		"Duracao em segundos do audio inbound por outcome",
		"s",
		[]float64{1, 5, 10, 20, 30, 45, 60},
	)
	costMicrousdTotal := o11y.Metrics().Counter(
		"agents_audio_cost_microusd_total",
		"Custo acumulado de transcricao de audio em microusd por provider/model",
		"1",
	)
	return &ProcessAudioInbound{
		media:                mediaClient,
		transcriber:          transcriber,
		audit:                audit,
		cfg:                  cfg,
		o11y:                 o11y,
		inboundTotal:         inboundTotal,
		transcriptionLatency: transcriptionLatency,
		downloadLatency:      downloadLatency,
		sizeBytes:            sizeBytes,
		durationSeconds:      durationSeconds,
		costMicrousdTotal:    costMicrousdTotal,
	}
}

type audioTerminalArgs struct {
	outcome             AudioOutcome
	reason              AudioReason
	errorCode           string
	size                *int64
	mimeType            string
	durationMs          *int
	sttModel            *string
	costMicrousd        *int64
	transcription       *string
	transcriptionSHA256 *string
}

func (p *ProcessAudioInbound) Execute(ctx context.Context, in AudioInboundInput) (AudioInboundResult, error) {
	ctx, span := p.o11y.Tracer().Start(ctx, "agents.usecase.process_audio_inbound")
	defer span.End()

	if err := validateAudioInboundInput(in); err != nil {
		span.RecordError(err)
		return AudioInboundResult{}, fmt.Errorf("%w: %w", ErrAudioInboundInvalidInput, err)
	}

	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return AudioInboundResult{}, fmt.Errorf("%w: user_id invalido", ErrAudioInboundInvalidInput)
	}

	now := time.Now().UTC()

	if result, found, findErr := p.checkExisting(ctx, in.WAMID, now); found {
		return result, findErr
	}

	if insertErr := p.audit.InsertProcessing(ctx, AudioAuditRecord{
		WAMID:       in.WAMID,
		UserID:      userID,
		Peer:        in.Peer,
		MediaID:     in.MediaID,
		MediaSHA256: in.SHA256,
		MimeType:    in.MimeType,
		Outcome:     AudioOutcomeProcessing,
		Reason:      AudioReasonProcessing,
		CreatedAt:   now,
	}); insertErr != nil {
		if errors.Is(insertErr, ErrAudioAuditAlreadyExists) {
			if result, found, findErr := p.checkExisting(ctx, in.WAMID, now); found {
				return result, findErr
			}
		}
		span.RecordError(insertErr)
		return AudioInboundResult{}, fmt.Errorf("agents.usecase.process_audio_inbound: abrir auditoria: %w", insertErr)
	}

	fetch, terminalResult, terminalErr, isTerminal := p.fetchAudio(ctx, in, userID, now)
	if isTerminal {
		return terminalResult, terminalErr
	}

	return p.transcribeAndDecide(ctx, in, userID, now, fetch)
}

type audioFetchResult struct {
	downloaded media.DownloadedMedia
	durationMs int
	format     llm.AudioFormat
}

func (p *ProcessAudioInbound) checkExisting(ctx context.Context, wamid string, now time.Time) (AudioInboundResult, bool, error) {
	existing, findErr := p.audit.FindByWAMID(ctx, wamid)
	if findErr == nil {
		if existing.Outcome == AudioOutcomeProcessing {
			return p.finalizeInterrupted(ctx, existing, now)
		}
		p.recordOutcome(ctx, existing.Outcome, existing.Reason)
		return p.resultFromRecord(existing), true, nil
	}
	if !errors.Is(findErr, ErrAudioAuditNotFound) {
		return AudioInboundResult{}, true, fmt.Errorf("agents.usecase.process_audio_inbound: find audit: %w", findErr)
	}
	return AudioInboundResult{}, false, nil
}

func (p *ProcessAudioInbound) finalizeInterrupted(
	ctx context.Context,
	existing AudioAuditRecord,
	now time.Time,
) (AudioInboundResult, bool, error) {
	if err := p.audit.UpdateOutcome(ctx, existing.WAMID, AudioOutcomeTranscriptionFailed, AudioReasonInterrupted, now); err != nil {
		return AudioInboundResult{}, true, fmt.Errorf("agents.usecase.process_audio_inbound: finalizar execucao interrompida: %w", err)
	}
	existing.Outcome = AudioOutcomeTranscriptionFailed
	existing.Reason = AudioReasonInterrupted
	p.recordOutcome(ctx, existing.Outcome, existing.Reason)
	p.o11y.Logger().Warn(ctx, "agents.usecase.process_audio_inbound: execucao anterior interrompida; wamid finalizado sem reprocessar",
		observability.String("outcome", existing.Outcome.String()),
		observability.String("reason", existing.Reason.String()),
	)
	return p.resultFromRecord(existing), true, nil
}

func (p *ProcessAudioInbound) fetchAudio(
	ctx context.Context,
	in AudioInboundInput,
	userID uuid.UUID,
	now time.Time,
) (audioFetchResult, AudioInboundResult, error, bool) {
	descriptor, err := p.media.Resolve(ctx, in.MediaID)
	if err != nil {
		result, terr := p.terminal(ctx, in, userID, now, audioTerminalArgs{
			outcome: AudioOutcomeRejected, reason: AudioReasonMediaUnavailable,
			errorCode: "media_resolve_failed", mimeType: in.MimeType,
		})
		return audioFetchResult{}, result, terr, true
	}

	if p.cfg.MaxBytes <= 0 {
		result, terr := p.terminal(ctx, in, userID, now, audioTerminalArgs{
			outcome: AudioOutcomeRejected, reason: AudioReasonInvalidPayload,
			errorCode: "max_bytes_not_configured", mimeType: in.MimeType,
		})
		return audioFetchResult{}, result, terr, true
	}

	dlStart := time.Now()
	downloaded, err := p.media.Download(ctx, descriptor.URL, p.cfg.MaxBytes)
	dlElapsed := time.Since(dlStart)
	if err != nil {
		p.recordDownloadLatency(ctx, audioDownloadOutcomeError, dlElapsed)
		reason := AudioReasonMediaUnavailable
		if errors.Is(err, media.ErrDownloadTooLarge) {
			reason = AudioReasonSizeExceeded
		}
		result, terr := p.terminal(ctx, in, userID, now, audioTerminalArgs{
			outcome: AudioOutcomeRejected, reason: reason,
			errorCode: classifyMediaDownloadError(err), mimeType: in.MimeType,
		})
		return audioFetchResult{}, result, terr, true
	}
	p.recordDownloadLatency(ctx, audioDownloadOutcomeSuccess, dlElapsed)

	if verifyErr := media.VerifyChecksum(descriptor, downloaded); verifyErr != nil {
		result, terr := p.terminal(ctx, in, userID, now, audioTerminalArgs{
			outcome: AudioOutcomeRejected, reason: AudioReasonMediaUnavailable, errorCode: "sha256_mismatch",
			size: &downloaded.Size, mimeType: downloaded.MimeType,
		})
		return audioFetchResult{}, result, terr, true
	}

	format, formatErr := audioFormatFromMime(downloaded.MimeType)
	if formatErr != nil {
		result, terr := p.terminal(ctx, in, userID, now, audioTerminalArgs{
			outcome: AudioOutcomeRejected, reason: AudioReasonInvalidPayload, errorCode: "format_unsupported",
			size: &downloaded.Size, mimeType: downloaded.MimeType,
		})
		return audioFetchResult{}, result, terr, true
	}

	duration, durErr := media.DetermineDuration(downloaded.MimeType, downloaded.Bytes)
	if durErr != nil {
		result, terr := p.terminal(ctx, in, userID, now, audioTerminalArgs{
			outcome: AudioOutcomeRejected, reason: AudioReasonDurationUnavailable, errorCode: "duration_undetermined",
			size: &downloaded.Size, mimeType: downloaded.MimeType,
		})
		return audioFetchResult{}, result, terr, true
	}

	durationMs := int(duration.Milliseconds())
	if maxDurErr := media.CheckMaxDuration(duration, p.cfg.MaxDuration); maxDurErr != nil {
		result, terr := p.terminal(ctx, in, userID, now, audioTerminalArgs{
			outcome: AudioOutcomeRejected, reason: AudioReasonDurationExceeded, errorCode: "duration_exceeded",
			size: &downloaded.Size, mimeType: downloaded.MimeType, durationMs: &durationMs,
		})
		return audioFetchResult{}, result, terr, true
	}

	return audioFetchResult{downloaded: downloaded, durationMs: durationMs, format: format}, AudioInboundResult{}, nil, false
}

func (p *ProcessAudioInbound) transcribeAndDecide(
	ctx context.Context,
	in AudioInboundInput,
	userID uuid.UUID,
	now time.Time,
	fetch audioFetchResult,
) (AudioInboundResult, error) {
	downloaded := fetch.downloaded
	durationMs := fetch.durationMs

	sttStart := time.Now()
	sttResp, sttErr := p.transcriber.Transcribe(ctx, llm.TranscriptionRequest{
		Model:           p.cfg.STTModel,
		Audio:           downloaded.Bytes,
		Format:          fetch.format,
		Language:        "pt",
		Temperature:     0,
		DurationMs:      durationMs,
		MaxCostMicrousd: p.cfg.MaxCostMicrousd,
	})
	sttElapsed := time.Since(sttStart)
	downloaded.Bytes = nil

	if sttErr != nil {
		return p.handleSTTError(ctx, in, userID, now, downloaded, durationMs, sttErr, sttElapsed)
	}

	decisionInput := input.AudioDecisionInput{
		Model:         p.cfg.STTModel,
		Text:          sttResp.Text,
		Language:      sttResp.Language,
		Truncated:     sttResp.Truncated,
		Confidence:    sttResp.Confidence,
		MinConfidence: p.cfg.MinConfidence,
	}
	if validateErr := decisionInput.Validate(); validateErr != nil {
		return AudioInboundResult{}, fmt.Errorf("agents.usecase.process_audio_inbound: decision input invalido: %w", validateErr)
	}
	decision := DecideAudioTranscription(decisionInput)
	p.recordTranscriptionLatency(ctx, p.cfg.STTModel, decision.Outcome, sttElapsed)

	sttModel := sttResp.Model
	args := audioTerminalArgs{
		outcome:    decision.Outcome,
		reason:     decision.Reason,
		size:       &downloaded.Size,
		mimeType:   downloaded.MimeType,
		durationMs: &durationMs,
		sttModel:   &sttModel,
	}
	if sttResp.CostMicrousd != nil {
		args.costMicrousd = sttResp.CostMicrousd
	}
	if decision.Outcome == AudioOutcomeApproved {
		sum := sha256.Sum256([]byte(decision.CanonicalText))
		hash := hex.EncodeToString(sum[:])
		text := decision.CanonicalText
		args.transcription = &text
		args.transcriptionSHA256 = &hash
	}

	return p.terminal(ctx, in, userID, now, args)
}

func (p *ProcessAudioInbound) handleSTTError(
	ctx context.Context,
	in AudioInboundInput,
	userID uuid.UUID,
	now time.Time,
	downloaded media.DownloadedMedia,
	durationMs int,
	sttErr error,
	sttElapsed time.Duration,
) (AudioInboundResult, error) {
	if errors.Is(sttErr, llm.ErrSTTCostPreflightExceeded) || errors.Is(sttErr, llm.ErrSTTCostExceeded) {
		p.recordTranscriptionLatency(ctx, p.cfg.STTModel, AudioOutcomeRejected, sttElapsed)
		return p.terminal(ctx, in, userID, now, audioTerminalArgs{
			outcome: AudioOutcomeRejected, reason: AudioReasonCostExceeded, errorCode: "cost_exceeded",
			size: &downloaded.Size, mimeType: downloaded.MimeType, durationMs: &durationMs, sttModel: &p.cfg.STTModel,
		})
	}
	decisionInput := input.AudioDecisionInput{
		Model:         p.cfg.STTModel,
		MinConfidence: p.cfg.MinConfidence,
		STTError:      sttErr,
	}
	if validateErr := decisionInput.Validate(); validateErr != nil {
		return AudioInboundResult{}, fmt.Errorf("agents.usecase.process_audio_inbound: decision input invalido: %w", validateErr)
	}
	decision := DecideAudioTranscription(decisionInput)
	p.recordTranscriptionLatency(ctx, p.cfg.STTModel, decision.Outcome, sttElapsed)
	return p.terminal(ctx, in, userID, now, audioTerminalArgs{
		outcome: decision.Outcome, reason: decision.Reason, errorCode: classifySTTError(sttErr),
		size: &downloaded.Size, mimeType: downloaded.MimeType, durationMs: &durationMs, sttModel: &p.cfg.STTModel,
	})
}

func (p *ProcessAudioInbound) MarkDispatched(ctx context.Context, wamid string) error {
	ctx, span := p.o11y.Tracer().Start(ctx, "agents.usecase.process_audio_inbound.mark_dispatched")
	defer span.End()

	if strings.TrimSpace(wamid) == "" {
		return fmt.Errorf("%w: wamid", ErrAudioInboundInvalidInput)
	}
	if err := p.audit.UpdateOutcome(ctx, wamid, AudioOutcomeDispatched, AudioReasonApproved, time.Now().UTC()); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents.usecase.process_audio_inbound: mark dispatched: %w", err)
	}
	p.recordOutcome(ctx, AudioOutcomeDispatched, AudioReasonApproved)
	return nil
}

func (p *ProcessAudioInbound) terminal(
	ctx context.Context,
	in AudioInboundInput,
	userID uuid.UUID,
	now time.Time,
	args audioTerminalArgs,
) (AudioInboundResult, error) {
	record := AudioAuditRecord{
		WAMID:               in.WAMID,
		UserID:              userID,
		Peer:                in.Peer,
		MediaID:             in.MediaID,
		MediaSHA256:         in.SHA256,
		MimeType:            firstNonEmpty(args.mimeType, in.MimeType),
		SizeBytes:           args.size,
		DurationMs:          args.durationMs,
		STTModel:            args.sttModel,
		Outcome:             args.outcome,
		Reason:              args.reason,
		Transcription:       args.transcription,
		TranscriptionSHA256: args.transcriptionSHA256,
		CostMicroUSD:        args.costMicrousd,
		ErrorCode:           nilIfEmpty(args.errorCode),
		CreatedAt:           now,
		CompletedAt:         &now,
	}

	if finalizeErr := p.audit.FinalizeTerminal(ctx, record); finalizeErr != nil {
		if errors.Is(finalizeErr, ErrAudioAuditNotFound) {
			existing, findErr := p.audit.FindByWAMID(ctx, in.WAMID)
			if findErr != nil {
				return AudioInboundResult{}, fmt.Errorf("agents.usecase.process_audio_inbound: recuperar audit apos finalizacao concorrente: %w", findErr)
			}
			p.recordOutcome(ctx, existing.Outcome, existing.Reason)
			return p.resultFromRecord(existing), nil
		}
		return AudioInboundResult{}, fmt.Errorf("agents.usecase.process_audio_inbound: persistir auditoria: %w", finalizeErr)
	}

	p.recordOutcome(ctx, args.outcome, args.reason)
	p.recordTerminalMetrics(ctx, args)
	p.logTerminal(ctx, args)
	return p.resultFromRecord(record), nil
}

func (p *ProcessAudioInbound) resultFromRecord(record AudioAuditRecord) AudioInboundResult {
	switch record.Outcome {
	case AudioOutcomeApproved:
		result := AudioInboundResult{Outcome: record.Outcome, Reason: record.Reason}
		if record.Transcription != nil {
			result.CanonicalText = *record.Transcription
		}
		return result
	case AudioOutcomeDispatched:
		return AudioInboundResult{Outcome: record.Outcome, Reason: record.Reason, ReplyText: defaultAudioAlreadyProcessedReply}
	default:
		return AudioInboundResult{Outcome: record.Outcome, Reason: record.Reason, ReplyText: p.replyFor(record.Outcome)}
	}
}

func (p *ProcessAudioInbound) replyFor(outcome AudioOutcome) string {
	switch outcome {
	case AudioOutcomeTranscriptionUncertain, AudioOutcomeTranscriptionFailed:
		return firstNonEmpty(p.cfg.UncertainReply, defaultAudioUncertainReply)
	default:
		return firstNonEmpty(p.cfg.RejectedReply, defaultAudioRejectedReply)
	}
}

func (p *ProcessAudioInbound) recordOutcome(ctx context.Context, outcome AudioOutcome, reason AudioReason) {
	p.inboundTotal.Add(ctx, 1,
		observability.String("channel", "whatsapp"),
		observability.String("outcome", outcome.String()),
		observability.String("reason", reason.String()),
	)
}

func (p *ProcessAudioInbound) recordDownloadLatency(ctx context.Context, outcome string, elapsed time.Duration) {
	p.downloadLatency.Record(ctx, elapsed.Seconds(),
		observability.String("provider", audioMediaProvider),
		observability.String("outcome", outcome),
	)
}

func (p *ProcessAudioInbound) recordTranscriptionLatency(ctx context.Context, model string, outcome AudioOutcome, elapsed time.Duration) {
	p.transcriptionLatency.Record(ctx, elapsed.Seconds(),
		observability.String("provider", audioSTTProvider),
		observability.String("model", model),
		observability.String("outcome", outcome.String()),
	)
}

func (p *ProcessAudioInbound) recordTerminalMetrics(ctx context.Context, args audioTerminalArgs) {
	if args.size != nil && *args.size > 0 {
		p.sizeBytes.Record(ctx, float64(*args.size),
			observability.String("mime_family", mimeFamily(args.mimeType)),
			observability.String("outcome", args.outcome.String()),
		)
	}
	if args.durationMs != nil {
		p.durationSeconds.Record(ctx, float64(*args.durationMs)/1000,
			observability.String("outcome", args.outcome.String()),
		)
	}
	if args.costMicrousd != nil {
		model := p.cfg.STTModel
		if args.sttModel != nil && *args.sttModel != "" {
			model = *args.sttModel
		}
		p.costMicrousdTotal.Add(ctx, *args.costMicrousd,
			observability.String("provider", audioSTTProvider),
			observability.String("model", model),
		)
	}
}

func (p *ProcessAudioInbound) logTerminal(ctx context.Context, args audioTerminalArgs) {
	fields := []observability.Field{
		observability.String("outcome", args.outcome.String()),
		observability.String("reason", args.reason.String()),
		observability.String("mime_family", mimeFamily(args.mimeType)),
	}
	if args.durationMs != nil {
		fields = append(fields, observability.Int("duration_ms", *args.durationMs))
	}
	if args.size != nil {
		fields = append(fields, observability.Int64("size_bytes", *args.size))
	}
	if args.sttModel != nil && *args.sttModel != "" {
		fields = append(fields, observability.String("stt_model", *args.sttModel))
	}
	if args.errorCode != "" {
		fields = append(fields, observability.String("error_code", args.errorCode))
	}

	if isAudioWarnOutcome(args.outcome, args.reason) {
		p.o11y.Logger().Warn(ctx, "agents.usecase.process_audio_inbound: audio processado", fields...)
		return
	}
	p.o11y.Logger().Info(ctx, "agents.usecase.process_audio_inbound: audio processado", fields...)
}

func isAudioWarnOutcome(outcome AudioOutcome, reason AudioReason) bool {
	switch outcome {
	case AudioOutcomeRejected, AudioOutcomeTranscriptionUncertain, AudioOutcomeTranscriptionFailed:
		return true
	default:
		return reason == AudioReasonCostExceeded
	}
}

func mimeFamily(mimeType string) string {
	base, _, _ := strings.Cut(mimeType, ";")
	switch strings.ToLower(strings.TrimSpace(base)) {
	case "audio/ogg", "audio/opus":
		return "ogg"
	case "audio/mp4", "audio/m4a", "audio/x-m4a", "audio/aac", "audio/mp4a-latm":
		return "m4a"
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func validateAudioInboundInput(in AudioInboundInput) error {
	var errs []error
	if strings.TrimSpace(in.WAMID) == "" {
		errs = append(errs, fmt.Errorf("wamid: required"))
	}
	if strings.TrimSpace(in.UserID) == "" {
		errs = append(errs, fmt.Errorf("user_id: required"))
	}
	if strings.TrimSpace(in.Peer) == "" {
		errs = append(errs, fmt.Errorf("peer: required"))
	}
	if strings.TrimSpace(in.MediaID) == "" {
		errs = append(errs, fmt.Errorf("media_id: required"))
	}
	if strings.TrimSpace(in.MimeType) == "" {
		errs = append(errs, fmt.Errorf("mime_type: required"))
	}
	if strings.TrimSpace(in.SHA256) == "" {
		errs = append(errs, fmt.Errorf("sha256: required"))
	}
	return errors.Join(errs...)
}

func audioFormatFromMime(mimeType string) (llm.AudioFormat, error) {
	base, _, _ := strings.Cut(mimeType, ";")
	switch strings.ToLower(strings.TrimSpace(base)) {
	case "audio/ogg", "audio/opus":
		return llm.AudioFormatOGG, nil
	case "audio/mp4", "audio/m4a", "audio/x-m4a", "audio/aac", "audio/mp4a-latm":
		return llm.AudioFormatM4A, nil
	default:
		return "", fmt.Errorf("agents.usecase.process_audio_inbound: mime type nao suportado: %s", base)
	}
}

func classifyMediaDownloadError(err error) string {
	switch {
	case errors.Is(err, media.ErrDownloadTooLarge):
		return "size_exceeded"
	case errors.Is(err, media.ErrMediaAuth):
		return "media_auth_failed"
	case errors.Is(err, media.ErrMediaServer):
		return "media_server_error"
	case errors.Is(err, media.ErrMediaBadRequest):
		return "media_bad_request"
	case errors.Is(err, media.ErrDownloadURLEmpty), errors.Is(err, media.ErrMediaURLMissing):
		return "media_url_missing"
	default:
		return "media_download_failed"
	}
}

func classifySTTError(err error) string {
	switch {
	case errors.Is(err, llm.ErrSTTTimeout):
		return "stt_timeout"
	case errors.Is(err, llm.ErrSTTModelUnavailable):
		return "stt_model_unavailable"
	case errors.Is(err, llm.ErrSTTEmptyText):
		return "stt_empty_text"
	case errors.Is(err, llm.ErrSTTInvalidResponse):
		return "stt_invalid_response"
	case errors.Is(err, llm.ErrSTTUpstream):
		return "stt_upstream_error"
	default:
		return "stt_error"
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
