package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type upsertUseCase interface {
	Execute(ctx context.Context, in input.UpsertUserByWhatsApp) (output.UpsertUserByWhatsApp, error)
}

type UpsertUserByWhatsAppHandler struct {
	usecase upsertUseCase
	o11y    observability.Observability
}

func NewUpsertUserByWhatsAppHandler(
	uc upsertUseCase,
	o11y observability.Observability,
) *UpsertUserByWhatsAppHandler {
	return &UpsertUserByWhatsAppHandler{usecase: uc, o11y: o11y}
}

type upsertUserRequest struct {
	WhatsApp    string `json:"whatsapp"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type upsertUserResponse struct {
	ID          string    `json:"id"`
	WhatsApp    string    `json:"whatsapp"`
	Email       string    `json:"email,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *UpsertUserByWhatsAppHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "identity.handler.upsert_user_by_whatsapp")
	defer span.End()

	var req upsertUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responses.ErrorWithDetails(w, http.StatusBadRequest, "JSON inválido",
			map[string]string{"code": "invalid_payload"})
		return
	}

	_, err := valueobjects.NewWhatsAppNumber(req.WhatsApp)
	if err != nil {
		responses.ErrorWithDetails(w, http.StatusBadRequest, "whatsapp inválido",
			map[string]string{"code": "invalid_whatsapp"})
		return
	}

	if req.Email != "" {
		if _, err = valueobjects.NewEmail(req.Email); err != nil {
			responses.ErrorWithDetails(w, http.StatusBadRequest, "email inválido",
				map[string]string{"code": "invalid_email"})
			return
		}
	}

	out, err := h.usecase.Execute(ctx, input.UpsertUserByWhatsApp{
		WhatsAppNumber: req.WhatsApp,
		Email:          req.Email,
		DisplayName:    req.DisplayName,
	})
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, application.ErrWhatsAppNumberInUse):
			responses.ErrorWithDetails(w, http.StatusConflict, "número já vinculado a outra conta",
				map[string]string{"code": "whatsapp_in_use"})
		case errors.Is(err, application.ErrEmailInUse):
			responses.ErrorWithDetails(w, http.StatusConflict, "email já vinculado a outra conta",
				map[string]string{"code": "email_in_use"})
		default:
			h.o11y.Logger().Error(ctx, "identity.handler.upsert_failed",
				observability.String("layer", "handler"),
				observability.String("operation", "upsert_user_by_whatsapp"),
				observability.Error(err),
			)
			responses.Error(w, http.StatusInternalServerError, "erro inesperado")
		}
		return
	}

	responses.JSON(w, http.StatusOK, upsertUserResponse{
		ID:          out.ID,
		WhatsApp:    out.WhatsAppNumber,
		Email:       out.Email,
		DisplayName: out.DisplayName,
		Status:      out.Status,
		CreatedAt:   out.CreatedAt,
		UpdatedAt:   out.UpdatedAt,
	})
}
