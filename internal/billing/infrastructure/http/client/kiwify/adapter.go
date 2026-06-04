package kiwify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

// ErrFetchSubscriptionFailed é retornado quando não é possível obter a subscription da Kiwify.
var ErrFetchSubscriptionFailed = errors.New("kiwify: falha ao obter subscription")

// KiwifyAdapter implementa a interface BillingProvider para a Kiwify.
// Delega para SignatureVerifier, PayloadMapper, OAuthClient e Client (ADR-006, ADR-008).
type KiwifyAdapter struct {
	client   *Client
	oauth    *OAuthClient
	verifier SignatureVerifier
	mapper   *PayloadMapper
	registry *BillingPlansRegistry
}

// NewKiwifyAdapter cria um KiwifyAdapter com todas as dependências injetadas.
func NewKiwifyAdapter(
	client *Client,
	oauth *OAuthClient,
	verifier SignatureVerifier,
	mapper *PayloadMapper,
	registry *BillingPlansRegistry,
) *KiwifyAdapter {
	return &KiwifyAdapter{
		client:   client,
		oauth:    oauth,
		verifier: verifier,
		mapper:   mapper,
		registry: registry,
	}
}

// VerifySignature delega a verificação de assinatura para o SignatureVerifier injetado.
// Retorna ErrMissingSignature ou ErrInvalidSignature em caso de falha (ADR-006).
func (a *KiwifyAdapter) VerifySignature(payload []byte, headers map[string]string) error {
	return a.verifier.Verify(payload, headers)
}

// ParseEvent mapeia o payload bruto Kiwify para CanonicalEvent.
// Delega para PayloadMapper (RF-29, RF-30).
func (a *KiwifyAdapter) ParseEvent(payload []byte) (services.CanonicalEvent, error) {
	return a.mapper.Parse(payload)
}

// FetchSubscription busca subscription na API Kiwify via GET /v1/sales/{order_id}.
// Realiza retry único em 401 com ForceRefresh do token OAuth (ADR-008).
func (a *KiwifyAdapter) FetchSubscription(ctx context.Context, externalSubscriptionID string) (services.CanonicalSubscription, error) {
	token, err := a.oauth.Token(ctx)
	if err != nil {
		return services.CanonicalSubscription{}, fmt.Errorf("kiwify fetch subscription: obter token: %w", err)
	}
	result, err := a.doFetchSubscription(ctx, externalSubscriptionID, token)
	if err == nil {
		return result, nil
	}
	if errors.Is(err, errRateLimited) {
		result, err = a.retryFetchSubscriptionAfterRateLimit(ctx, externalSubscriptionID, token)
		if err != nil {
			return services.CanonicalSubscription{}, fmt.Errorf("kiwify fetch subscription: %w: %w", ErrFetchSubscriptionFailed, err)
		}
		return result, nil
	}
	if !errors.Is(err, errUnauthorized) {
		return services.CanonicalSubscription{}, fmt.Errorf("kiwify fetch subscription: %w: %w", ErrFetchSubscriptionFailed, err)
	}
	// retry único em 401 — força refresh do token (ADR-008)
	token, err = a.oauth.ForceRefresh(ctx)
	if err != nil {
		return services.CanonicalSubscription{}, fmt.Errorf("kiwify fetch subscription: force refresh: %w", err)
	}
	result, err = a.doFetchSubscription(ctx, externalSubscriptionID, token)
	if err != nil {
		return services.CanonicalSubscription{}, fmt.Errorf("kiwify fetch subscription: %w: %w", ErrFetchSubscriptionFailed, err)
	}
	return result, nil
}

var errUnauthorized = errors.New("unauthorized")
var errRateLimited = errors.New("rate limited")

func (a *KiwifyAdapter) retryFetchSubscriptionAfterRateLimit(
	ctx context.Context,
	orderID string,
	token string,
) (services.CanonicalSubscription, error) {
	for _, backoff := range a.client.backoffs {
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return services.CanonicalSubscription{}, ctx.Err()
		}
		result, err := a.doFetchSubscription(ctx, orderID, token)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, errRateLimited) {
			return services.CanonicalSubscription{}, err
		}
	}
	return services.CanonicalSubscription{}, errRateLimited
}

func (a *KiwifyAdapter) doFetchSubscription(ctx context.Context, orderID string, token string) (services.CanonicalSubscription, error) {
	req, err := a.client.newRequest(ctx, http.MethodGet, "/v1/sales/"+orderID, nil)
	if err != nil {
		return services.CanonicalSubscription{}, fmt.Errorf("construir request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.client.do(ctx, req)
	if err != nil {
		return services.CanonicalSubscription{}, fmt.Errorf("executar request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests {
		return services.CanonicalSubscription{}, errRateLimited
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return services.CanonicalSubscription{}, errUnauthorized
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return services.CanonicalSubscription{}, fmt.Errorf("servidor retornou %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return services.CanonicalSubscription{}, fmt.Errorf("status inesperado: %d", resp.StatusCode)
	}
	var body kiwifySaleResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return services.CanonicalSubscription{}, fmt.Errorf("decodificar resposta: %w", err)
	}
	plan, err := a.registry.ParsePlanCodeFromKiwifyProductID(body.Product.ID)
	if err != nil {
		return services.CanonicalSubscription{}, fmt.Errorf("resolver plan code: %w", err)
	}
	status := mapKiwifyStatusToCanonical(body.Status)
	return services.CanonicalSubscription{
		ExternalID:  body.ID,
		Status:      status,
		PlanCode:    plan,
		PeriodStart: body.Subscription.CurrentPeriodStart,
		PeriodEnd:   body.Subscription.CurrentPeriodEnd,
		Customer: services.CanonicalCustomer{
			WhatsApp: normalizeOptionalWhatsApp(body.Customer.Mobile),
			Email:    body.Customer.Email,
		},
	}, nil
}

func mapKiwifyStatusToCanonical(status string) valueobjects.SubscriptionStatus {
	switch status {
	case "active":
		return valueobjects.SubscriptionStatusActive
	case "trialing":
		return valueobjects.SubscriptionStatusTrialing
	case "past_due", "late":
		return valueobjects.SubscriptionStatusPastDue
	case "canceled", "canceled_pending":
		return valueobjects.SubscriptionStatusCanceledPending
	case "expired":
		return valueobjects.SubscriptionStatusExpired
	case "refunded", "chargeback":
		return valueobjects.SubscriptionStatusRefunded
	default:
		return valueobjects.SubscriptionStatusUnknown
	}
}

type kiwifySaleResponse struct {
	ID           string         `json:"id"`
	Status       string         `json:"status"`
	Product      kiwifyProduct  `json:"product"`
	Customer     kiwifyCustomer `json:"customer"`
	Subscription struct {
		CurrentPeriodStart time.Time `json:"current_period_start"`
		CurrentPeriodEnd   time.Time `json:"current_period_end"`
	} `json:"subscription"`
}

func normalizeOptionalWhatsApp(raw string) identityvo.WhatsAppNumber {
	whatsapp, err := identityvo.NewWhatsAppNumber(raw)
	if err != nil {
		return identityvo.WhatsAppNumber{}
	}
	return whatsapp
}
