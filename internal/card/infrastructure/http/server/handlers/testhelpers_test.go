package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

var errUnexpected = errors.New("unexpected database error")

func ctxWithPrincipal(userID uuid.UUID) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: userID,
		Source: auth.SourceHeader,
	})
}

func sampleCard(userID uuid.UUID) output.Card {
	return output.Card{
		ID:         uuid.New().String(),
		UserID:     userID.String(),
		Name:       "Nubank",
		Nickname:   "Nu",
		ClosingDay: 15,
		DueDay:     22,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
