package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	prefixEstablishPrincipal  = "identity.usecase.establish_principal:"
	authSourceWhatsApp        = "whatsapp"
	reasonOutboxPublishFailed = "outbox_publish_failed"
	reasonDBUnavailable       = "db_unavailable"
	reasonInternalError       = "internal_error"
)

type EstablishResult struct {
	Principal auth.Principal
	Found     bool
}

type errOutboxPublish struct{ wrapped error }

func (e errOutboxPublish) Error() string { return e.wrapped.Error() }
func (e errOutboxPublish) Unwrap() error { return e.wrapped }

type errLookup struct{ wrapped error }

func (e errLookup) Error() string { return e.wrapped.Error() }
func (e errLookup) Unwrap() error { return e.wrapped }

func classifyEstablishErrorReason(err error) string {
	if _, ok := errors.AsType[errOutboxPublish](err); ok {
		return reasonOutboxPublishFailed
	}
	if _, ok := errors.AsType[errLookup](err); ok {
		return reasonDBUnavailable
	}
	return reasonInternalError
}

func eventOutboxFromDecision(decision services.PrincipalDecision, requestID, clientIP string) (outbox.Event, error) {
	userID := ""
	if decision.Found {
		userID = decision.UserID.String()
	}
	return newAuthEventOutbox(decision.EventID.String(), userID, string(decision.EventKind), authSourceWhatsApp, "", requestID, clientIP, decision.OccurredAt)
}

type EstablishPrincipal struct {
	uow              uow.UnitOfWork
	factory          interfaces.RepositoryFactory
	publisher        outbox.Publisher
	workflow         services.PrincipalWorkflow
	o11y             observability.Observability
	establishedTotal observability.Counter
	failedTotal      observability.Counter
	unknownTotal     observability.Counter
	resolveDuration  observability.Histogram
	resolvePathTotal observability.Counter
}

func NewEstablishPrincipal(
	u uow.UnitOfWork,
	factory interfaces.RepositoryFactory,
	publisher outbox.Publisher,
	o11y observability.Observability,
) *EstablishPrincipal {
	establishedTotal := o11y.Metrics().Counter(
		"auth_principal_established_total",
		"Total de Principals estabelecidos com sucesso por source",
		"1",
	)
	failedTotal := o11y.Metrics().Counter(
		"auth_failed_total",
		"Total de falhas de autenticacao por reason",
		"1",
	)
	unknownTotal := o11y.Metrics().Counter(
		"auth_unknown_wa_id_total",
		"Total de mensagens de wa_id desconhecido (sem usuario ativo)",
		"1",
	)
	resolveDuration := o11y.Metrics().HistogramWithBuckets(
		"auth_resolve_wa_duration_seconds",
		"Duracao da resolucao wa_id para user_id",
		"s",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	)
	resolvePathTotal := o11y.Metrics().Counter(
		"auth_resolve_path_total",
		"Caminho de resolucao do principal (identity vs legacy) durante migracao multi-canal",
		"1",
	)
	return &EstablishPrincipal{
		uow:              u,
		factory:          factory,
		publisher:        publisher,
		workflow:         services.PrincipalWorkflow{},
		o11y:             o11y,
		establishedTotal: establishedTotal,
		failedTotal:      failedTotal,
		unknownTotal:     unknownTotal,
		resolveDuration:  resolveDuration,
		resolvePathTotal: resolvePathTotal,
	}
}

func (u *EstablishPrincipal) Execute(ctx context.Context, in input.EstablishPrincipalInput) (auth.Principal, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "auth.resolve_principal",
		observability.WithAttributes(observability.String("source", string(auth.SourceWhatsApp))),
	)
	defer span.End()

	start := time.Now()

	wa, err := valueobjects.NewWhatsAppNumber(in.WhatsAppNumber)
	if err != nil {
		return auth.Principal{}, fmt.Errorf("%s parse whatsapp: %w", prefixEstablishPrincipal, err)
	}

	rid, err := u.resolveRequestID(in.RequestID, span.TraceID())
	if err != nil {
		return auth.Principal{}, fmt.Errorf("%s parse request_id: %w", prefixEstablishPrincipal, err)
	}

	cip := u.resolveClientIP(ctx, in.ClientIPRaw)

	res, err := uow.Do(ctx, u.uow, func(ctx context.Context, tx database.DBTX) (EstablishResult, error) {
		return u.resolvePrincipal(ctx, tx, wa, rid, cip)
	})

	elapsed := time.Since(start).Seconds()
	u.resolveDuration.Record(ctx, elapsed)

	if err != nil {
		reason := classifyEstablishErrorReason(err)
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", "error"))
		u.failedTotal.Add(ctx, 1, observability.String("reason", reason))
		u.o11y.Logger().Error(ctx, "identity.usecase.establish_principal_failed",
			observability.String("layer", "usecase"),
			observability.String("operation", "establish_principal"),
			observability.String("whatsapp", wa.Masked()),
			observability.String("request_id", rid.String()),
			observability.String("client_ip", cip.String()),
			observability.Error(err),
		)
		return auth.Principal{}, fmt.Errorf("%s %w", prefixEstablishPrincipal, err)
	}

	if !res.Found {
		span.SetAttributes(observability.String("outcome", "unknown"))
		u.unknownTotal.Add(ctx, 1)
		return auth.Principal{}, application.ErrUnknownUser
	}

	span.SetAttributes(
		observability.String("outcome", "found"),
		observability.String("user_id", res.Principal.UserID.String()),
	)
	u.establishedTotal.Add(ctx, 1, observability.String("source", string(auth.SourceWhatsApp)))
	return res.Principal, nil
}

func (u *EstablishPrincipal) resolvePrincipal(
	ctx context.Context,
	tx database.DBTX,
	wa valueobjects.WhatsAppNumber,
	rid valueobjects.RequestID,
	cip valueobjects.ClientIP,
) (EstablishResult, error) {
	userID, found, lookupErr := u.lookupUserIDByWhatsApp(ctx, tx, wa)
	if lookupErr != nil {
		return EstablishResult{}, errLookup{wrapped: fmt.Errorf("lookup: %w", lookupErr)}
	}

	eventID, err := uuid.NewV7()
	if err != nil {
		return EstablishResult{}, fmt.Errorf("generate auth event id: %w", err)
	}
	now := time.Now().UTC()

	decision := u.workflow.DecidePrincipal(userID, found, eventID, now)

	ev, buildErr := eventOutboxFromDecision(decision, rid.String(), cip.String())
	if buildErr != nil {
		return EstablishResult{}, buildErr
	}
	if pubErr := u.publishAuthOutcome(ctx, ev); pubErr != nil {
		return EstablishResult{}, pubErr
	}

	if !decision.Found {
		return EstablishResult{Found: false}, nil
	}
	return EstablishResult{
		Principal: auth.Principal{UserID: decision.UserID, Source: auth.SourceWhatsApp},
		Found:     true,
	}, nil
}

func (u *EstablishPrincipal) lookupUserIDByWhatsApp(
	ctx context.Context,
	tx database.DBTX,
	wa valueobjects.WhatsAppNumber,
) (uuid.UUID, bool, error) {
	channel := valueobjects.ChannelWhatsApp()
	externalID, err := valueobjects.NewExternalID(channel, wa.String())
	if err == nil {
		identityRepo := u.factory.UserIdentityRepository(tx)
		identity, identityFound, identityErr := identityRepo.TryFindActive(ctx, channel, externalID)
		if identityErr != nil {
			return uuid.Nil, false, fmt.Errorf("identity lookup: %w", identityErr)
		}
		if identityFound {
			u.resolvePathTotal.Add(ctx, 1, observability.String("path", "identity"))
			return identity.UserID(), true, nil
		}
	}

	userRepo := u.factory.UserRepository(tx)
	user, found, lookupErr := userRepo.TryFindActiveByWhatsApp(ctx, wa)
	if lookupErr != nil {
		return uuid.Nil, false, lookupErr
	}
	if !found {
		u.resolvePathTotal.Add(ctx, 1, observability.String("path", "miss"))
		return uuid.Nil, false, nil
	}

	parsed, parseErr := uuid.Parse(user.ID())
	if parseErr != nil {
		return uuid.Nil, false, fmt.Errorf("parse user id: %w", parseErr)
	}
	u.resolvePathTotal.Add(ctx, 1, observability.String("path", "legacy"))
	return parsed, true, nil
}

func (u *EstablishPrincipal) publishAuthOutcome(ctx context.Context, ev outbox.Event) error {
	if err := u.publisher.Publish(ctx, ev); err != nil {
		return errOutboxPublish{wrapped: fmt.Errorf("publish %s: %w", ev.Type, err)}
	}
	return nil
}

func (u *EstablishPrincipal) resolveRequestID(raw, fallback string) (valueobjects.RequestID, error) {
	if raw != "" {
		return valueobjects.NewRequestID(raw)
	}
	if fallback == "" {
		return valueobjects.RequestID{}, nil
	}
	return valueobjects.NewRequestID(fallback)
}

func (u *EstablishPrincipal) resolveClientIP(ctx context.Context, raw string) valueobjects.ClientIP {
	cip, err := valueobjects.NewClientIP(raw)
	if err == nil {
		return cip
	}
	u.o11y.Logger().Warn(ctx, "identity.usecase.establish_principal.invalid_client_ip",
		observability.String("client_ip_raw", raw),
		observability.Error(err),
	)
	return valueobjects.ClientIP{}
}
