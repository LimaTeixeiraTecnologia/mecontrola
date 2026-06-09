package idempotency_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	idemMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

func BenchmarkMiddlewareMiss(b *testing.B) {
	storage := idemMocks.NewStorage(b)
	storage.EXPECT().
		Get(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Times(b.N)

	o11y := noop.NewProvider()
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	mw := idempotency.Middleware("card", storage, 24*time.Hour, o11y)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	})
	handler := mw(next)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r := httptest.NewRequest(http.MethodPost, "/test", nil)
			r = r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{
				UserID: userID,
				Source: auth.SourceWhatsApp,
			}))
			r.Header.Set("Idempotency-Key", "bench-key")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
		}
	})
}
