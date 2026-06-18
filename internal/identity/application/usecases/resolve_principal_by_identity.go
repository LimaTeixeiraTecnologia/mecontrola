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
)

const prefixResolvePrincipalByIdentity = "identity.usecase.resolve_principal_by_identity:"

type ResolvePrincipalByIdentity struct {
	uow             uow.UnitOfWork
	factory         interfaces.RepositoryFactory
	workflow        services.IdentityResolutionWorkflow
	o11y            observability.Observability
	resolvedTotal   observability.Counter
	unknownTotal    observability.Counter
	mismatchTotal   observability.Counter
	unlinkedTotal   observability.Counter
	resolveDuration observability.Histogram
}

func NewResolvePrincipalByIdentity(
	u uow.UnitOfWork,
	factory interfaces.RepositoryFactory,
	o11y observability.Observability,
) *ResolvePrincipalByIdentity {
	resolvedTotal := o11y.Metrics().Counter(
		"identity_resolution_resolved_total",
		"Total de identidades resolvidas com sucesso por canal",
		"1",
	)
	unknownTotal := o11y.Metrics().Counter(
		"identity_resolution_unknown_total",
		"Total de tentativas de resolucao para (channel, external_id) sem registro",
		"1",
	)
	mismatchTotal := o11y.Metrics().Counter(
		"identity_resolution_mismatch_total",
		"Total de tentativas com mismatch entre identidade armazenada e candidate",
		"1",
	)
	unlinkedTotal := o11y.Metrics().Counter(
		"identity_resolution_unlinked_total",
		"Total de tentativas para identidade desvinculada",
		"1",
	)
	resolveDuration := o11y.Metrics().HistogramWithBuckets(
		"identity_resolution_duration_seconds",
		"Duracao da resolucao de identidade multi-canal",
		"s",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	)
	return &ResolvePrincipalByIdentity{
		uow:             u,
		factory:         factory,
		workflow:        services.IdentityResolutionWorkflow{},
		o11y:            o11y,
		resolvedTotal:   resolvedTotal,
		unknownTotal:    unknownTotal,
		mismatchTotal:   mismatchTotal,
		unlinkedTotal:   unlinkedTotal,
		resolveDuration: resolveDuration,
	}
}

func (u *ResolvePrincipalByIdentity) Execute(ctx context.Context, in input.ResolvePrincipalByIdentity) (auth.Principal, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.resolve_principal_by_identity")
	defer span.End()

	start := time.Now()

	channel, err := valueobjects.NewChannel(in.Channel)
	if err != nil {
		span.RecordError(err)
		return auth.Principal{}, fmt.Errorf("%s parse channel: %w", prefixResolvePrincipalByIdentity, err)
	}

	externalID, err := valueobjects.NewExternalID(channel, in.ExternalID)
	if err != nil {
		span.RecordError(err)
		return auth.Principal{}, fmt.Errorf("%s parse external_id: %w", prefixResolvePrincipalByIdentity, err)
	}

	span.SetAttributes(
		observability.String("channel", channel.String()),
		observability.String("external_id_masked", externalID.Masked()),
	)

	res, execErr := uow.Do(ctx, u.uow, func(ctx context.Context, tx database.DBTX) (EstablishResult, error) {
		return u.resolve(ctx, tx, channel, externalID)
	})

	elapsed := time.Since(start).Seconds()
	u.resolveDuration.Record(ctx, elapsed, observability.String("channel", channel.String()))

	if execErr != nil {
		span.RecordError(execErr)
		span.SetAttributes(observability.String("outcome", "error"))
		u.o11y.Logger().Error(ctx, "identity.usecase.resolve_principal_by_identity.failed",
			observability.String("layer", "usecase"),
			observability.String("operation", "resolve_principal_by_identity"),
			observability.String("channel", channel.String()),
			observability.String("external_id_masked", externalID.Masked()),
			observability.Error(execErr),
		)
		return auth.Principal{}, fmt.Errorf("%s %w", prefixResolvePrincipalByIdentity, execErr)
	}

	if !res.Found {
		span.SetAttributes(observability.String("outcome", "unknown"))
		return auth.Principal{}, application.ErrUnknownUser
	}

	span.SetAttributes(
		observability.String("outcome", "resolved"),
		observability.String("user_id", res.Principal.UserID.String()),
	)
	u.resolvedTotal.Add(ctx, 1, observability.String("channel", channel.String()))
	return res.Principal, nil
}

func (u *ResolvePrincipalByIdentity) resolve(
	ctx context.Context,
	tx database.DBTX,
	channel valueobjects.Channel,
	externalID valueobjects.ExternalID,
) (EstablishResult, error) {
	repo := u.factory.UserIdentityRepository(tx)

	candidate, found, lookupErr := repo.TryFindActive(ctx, channel, externalID)
	if lookupErr != nil {
		return EstablishResult{}, fmt.Errorf("lookup: %w", lookupErr)
	}

	eventID, err := uuid.NewV7()
	if err != nil {
		return EstablishResult{}, fmt.Errorf("generate event id: %w", err)
	}
	now := time.Now().UTC()

	decision := u.workflow.DecideResolve(candidate, found, channel, externalID, eventID, now)

	switch decision.Kind {
	case services.IdentityResolutionResolved:
		source, srcErr := auth.SourceFromChannel(channel.String())
		if srcErr != nil {
			u.o11y.Logger().Warn(ctx, "identity.usecase.resolve_principal_by_identity.unknown_channel",
				observability.String("channel", channel.String()),
				observability.Error(srcErr),
			)
		}
		return EstablishResult{
			Principal: auth.Principal{UserID: decision.UserID, Source: source},
			Found:     true,
		}, nil
	case services.IdentityResolutionUnknown:
		u.unknownTotal.Add(ctx, 1, observability.String("channel", channel.String()))
		return EstablishResult{Found: false}, nil
	case services.IdentityResolutionUnlinked:
		u.unlinkedTotal.Add(ctx, 1, observability.String("channel", channel.String()))
		return EstablishResult{Found: false}, nil
	case services.IdentityResolutionMismatch:
		u.mismatchTotal.Add(ctx, 1, observability.String("channel", channel.String()))
		return EstablishResult{}, errors.New("identity: resolution mismatch")
	default:
		return EstablishResult{}, fmt.Errorf("identity: unexpected resolution kind %d", decision.Kind)
	}
}
