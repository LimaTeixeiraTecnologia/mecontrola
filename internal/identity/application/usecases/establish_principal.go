package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	prefixEstablishPrincipal   = "identity.usecase.establish_principal:"
	authSourceWhatsApp         = "whatsapp"
	authKindPrincipalEstablish = "principal_established"
	authKindUnknownUser        = "unknown_user"
	reasonOutboxPublishFailed  = "outbox_publish_failed"
	reasonDBUnavailable        = "db_unavailable"
	reasonInternalError        = "internal_error"
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

func newPrincipalEstablishedEvent(userID string, now time.Time) (outbox.Event, error) {
	eid, err := uuid.NewV7()
	if err != nil {
		return outbox.Event{}, fmt.Errorf("generate principal_established event id: %w", err)
	}
	ev, err := newAuthEventOutbox(eid.String(), userID, authKindPrincipalEstablish, authSourceWhatsApp, "", now)
	if err != nil {
		return outbox.Event{}, fmt.Errorf("build principal_established event: %w", err)
	}
	return ev, nil
}

func newUnknownUserEvent(now time.Time) (outbox.Event, error) {
	eid, err := uuid.NewV7()
	if err != nil {
		return outbox.Event{}, fmt.Errorf("generate unknown_user event id: %w", err)
	}
	ev, err := newAuthEventOutbox(eid.String(), "", authKindUnknownUser, authSourceWhatsApp, "", now)
	if err != nil {
		return outbox.Event{}, fmt.Errorf("build unknown_user event: %w", err)
	}
	return ev, nil
}

type EstablishPrincipal struct {
	uow              uow.UnitOfWork[EstablishResult]
	factory          interfaces.RepositoryFactory
	publisher        outbox.Publisher
	o11y             observability.Observability
	establishedTotal observability.Counter
	failedTotal      observability.Counter
	unknownTotal     observability.Counter
	resolveDuration  observability.Histogram
}

func NewEstablishPrincipal(
	u uow.UnitOfWork[EstablishResult],
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
	return &EstablishPrincipal{
		uow:              u,
		factory:          factory,
		publisher:        publisher,
		o11y:             o11y,
		establishedTotal: establishedTotal,
		failedTotal:      failedTotal,
		unknownTotal:     unknownTotal,
		resolveDuration:  resolveDuration,
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

	res, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (EstablishResult, error) {
		return u.resolvePrincipal(ctx, tx, wa)
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
) (EstablishResult, error) {
	userRepo := u.factory.UserRepository(tx)

	user, found, lookupErr := userRepo.TryFindActiveByWhatsApp(ctx, wa)
	if lookupErr != nil {
		return EstablishResult{}, errLookup{wrapped: fmt.Errorf("lookup: %w", lookupErr)}
	}

	now := time.Now().UTC()

	if !found {
		ev, buildErr := newUnknownUserEvent(now)
		if buildErr != nil {
			return EstablishResult{}, buildErr
		}
		if pubErr := u.publishAuthOutcome(ctx, ev); pubErr != nil {
			return EstablishResult{}, pubErr
		}
		return EstablishResult{Found: false}, nil
	}

	userID := user.ID()
	ev, buildErr := newPrincipalEstablishedEvent(userID, now)
	if buildErr != nil {
		return EstablishResult{}, buildErr
	}
	if pubErr := u.publishAuthOutcome(ctx, ev); pubErr != nil {
		return EstablishResult{}, pubErr
	}

	uid, parseErr := uuid.Parse(userID)
	if parseErr != nil {
		return EstablishResult{}, fmt.Errorf("parse user id: %w", parseErr)
	}
	return EstablishResult{
		Principal: auth.Principal{UserID: uid, Source: auth.SourceWhatsApp},
		Found:     true,
	}, nil
}

func (u *EstablishPrincipal) publishAuthOutcome(ctx context.Context, ev outbox.Event) error {
	if err := u.publisher.Publish(ctx, ev); err != nil {
		return errOutboxPublish{wrapped: fmt.Errorf("publish %s: %w", ev.Type, err)}
	}
	return nil
}
