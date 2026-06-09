package signature_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
)

type ComposeSuite struct {
	suite.Suite
}

func TestComposeSuite(t *testing.T) {
	suite.Run(t, new(ComposeSuite))
}

func (s *ComposeSuite) buildSig(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (s *ComposeSuite) TestCompose_OrderIsRawBodyThenHMAC() {
	// Validates that raw body is available in context (required by HMAC middleware)
	// and that a valid signature passes through.
	payload := []byte(`{"object":"whatsapp_business_account"}`)
	secret := "test-secret"

	var rawBodyAvailable bool
	var hmacStatusInCtx string

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := signature.RawBodyFromContext(r)
		rawBodyAvailable = ok && len(raw) > 0
		hmacStatusInCtx = signature.StatusFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	handler := signature.Compose(secret, "", nil)(inner)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", s.buildSig(payload, secret))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.True(rawBodyAvailable, "raw body must be available in context — RawBody must run before HMAC")
	s.Equal(signature.StatusValid, hmacStatusInCtx)
}

func (s *ComposeSuite) TestCompose_InvalidSignatureReturns401() {
	payload := []byte(`{"object":"whatsapp_business_account"}`)
	secret := "test-secret"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := signature.Compose(secret, "", nil)(inner)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsig")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func (s *ComposeSuite) TestComposeWithStatus_CallbackCountsPerScenario() {
	payload := []byte(`{"object":"whatsapp_business_account"}`)

	scenarios := []struct {
		name             string
		secretCurrent    string
		secretNext       string
		header           string
		expectStatuses   []string
		expectInvalidCnt int
	}{
		{
			name:             "valid emits onStatus once with valid and never onInvalid",
			secretCurrent:    "current",
			header:           s.buildSig(payload, "current"),
			expectStatuses:   []string{signature.StatusValid},
			expectInvalidCnt: 0,
		},
		{
			name:             "rotated emits onStatus once with rotated and never onInvalid",
			secretCurrent:    "current",
			secretNext:       "next",
			header:           s.buildSig(payload, "next"),
			expectStatuses:   []string{signature.StatusRotated},
			expectInvalidCnt: 0,
		},
		{
			name:             "invalid emits onStatus once with invalid and onInvalid once",
			secretCurrent:    "current",
			header:           "sha256=deadbeef",
			expectStatuses:   []string{signature.StatusInvalid},
			expectInvalidCnt: 1,
		},
		{
			name:             "missing header emits onStatus once with invalid and onInvalid once",
			secretCurrent:    "current",
			header:           "",
			expectStatuses:   []string{signature.StatusInvalid},
			expectInvalidCnt: 1,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			var gotStatuses []string
			invalidCnt := 0
			onInvalid := func() { invalidCnt++ }
			onStatus := func(st string) { gotStatuses = append(gotStatuses, st) }

			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			handler := signature.ComposeWithStatus(sc.secretCurrent, sc.secretNext, onInvalid, onStatus)(inner)

			req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
			if sc.header != "" {
				req.Header.Set("X-Hub-Signature-256", sc.header)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			s.Equal(sc.expectStatuses, gotStatuses, "onStatus must be called exactly once with the resolved status")
			s.Equal(sc.expectInvalidCnt, invalidCnt, "onInvalid must fire only on invalid status")
		})
	}
}

func (s *ComposeSuite) TestCompose_CallsOnInvalidWhenSignatureInvalid() {
	payload := []byte(`{"object":"whatsapp_business_account"}`)
	secret := "test-secret"

	called := false
	onInvalid := func() { called = true }

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := signature.Compose(secret, "", onInvalid)(inner)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsig")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	s.True(called, "onInvalid callback must be called on invalid signature")
}
