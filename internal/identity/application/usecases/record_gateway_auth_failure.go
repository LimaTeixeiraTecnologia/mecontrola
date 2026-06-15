package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

var ErrInvalidGatewayReason = errors.New("identity: invalid gateway auth failure reason")

var gatewayReasons = map[entities.AuthEventReason]struct{}{
	entities.AuthEventReasonGatewayMissingHeader:    {},
	entities.AuthEventReasonGatewayInvalidTimestamp: {},
	entities.AuthEventReasonGatewayStaleTimestamp:   {},
	entities.AuthEventReasonGatewayInvalidSignature: {},
}

const prefixRecordGatewayAuthFailure = "identity.usecase.record_gateway_auth_failure:"

type RecordGatewayAuthFailure struct {
	publisher outbox.Publisher
	o11y      observability.Observability
}

func NewRecordGatewayAuthFailure(
	publisher outbox.Publisher,
	o11y observability.Observability,
) *RecordGatewayAuthFailure {
	return &RecordGatewayAuthFailure{publisher: publisher, o11y: o11y}
}

func (u *RecordGatewayAuthFailure) Handle(ctx context.Context, in input.RecordGatewayAuthFailureInput) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.record_gateway_auth_failure")
	defer span.End()

	reason := entities.AuthEventReason(in.Reason)
	if _, ok := gatewayReasons[reason]; !ok {
		return fmt.Errorf("%s %w: %q", prefixRecordGatewayAuthFailure, ErrInvalidGatewayReason, in.Reason)
	}

	var rid valueobjects.RequestID
	var err error
	if in.RequestID != "" {
		rid, err = valueobjects.NewRequestID(in.RequestID)
		if err != nil {
			return fmt.Errorf("%s parse request_id: %w", prefixRecordGatewayAuthFailure, err)
		}
	}

	cip := u.resolveClientIP(ctx, in.ClientIPRaw)

	eventID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("%s generate event_id: %w", prefixRecordGatewayAuthFailure, err)
	}

	now := time.Now().UTC()

	var userID *uuid.UUID
	if in.UserIDRaw != "" {
		uid, parseErr := uuid.Parse(in.UserIDRaw)
		if parseErr != nil {
			u.o11y.Logger().Warn(ctx, "identity.usecase.record_gateway_auth_failure.invalid_user_id",
				observability.String("reason", string(reason)),
				observability.String("request_id", rid.String()),
				observability.String("client_ip", cip.String()),
				observability.Error(parseErr),
			)
		} else {
			userID = &uid
		}
	}

	userIDStr := ""
	if userID != nil {
		userIDStr = userID.String()
	}

	ev, err := newAuthEventOutbox(
		eventID.String(),
		userIDStr,
		string(entities.AuthEventKindFailed),
		string(entities.AuthEventSourceGateway),
		string(reason),
		rid.String(),
		cip.String(),
		now,
	)
	if err != nil {
		return fmt.Errorf("%s build outbox event: %w", prefixRecordGatewayAuthFailure, err)
	}

	if err := u.publisher.Publish(ctx, ev); err != nil {
		return fmt.Errorf("%s publish: %w", prefixRecordGatewayAuthFailure, err)
	}
	return nil
}

func (u *RecordGatewayAuthFailure) resolveClientIP(ctx context.Context, raw string) valueobjects.ClientIP {
	cip, err := valueobjects.NewClientIP(raw)
	if err == nil {
		return cip
	}
	u.o11y.Logger().Warn(ctx, "identity.usecase.record_gateway_auth_failure.invalid_client_ip",
		observability.String("client_ip_raw", raw),
		observability.Error(err),
	)
	return valueobjects.ClientIP{}
}
