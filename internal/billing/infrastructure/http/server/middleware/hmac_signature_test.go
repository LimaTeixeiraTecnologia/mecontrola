package middleware_test

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
)

const (
	testSecret     = "secret-current"
	testSecretNext = "secret-next"
)

type HmacSignatureSuite struct {
	suite.Suite
}

func TestHmacSignatureSuite(t *testing.T) {
	suite.Run(t, new(HmacSignatureSuite))
}

func (s *HmacSignatureSuite) SetupTest() {}

func buildSignature(body []byte, secret string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func newHandler(secretCurrent, secretNext string) http.Handler {
	return middleware.RawBody(
		middleware.HMACSignature(secretCurrent, secretNext)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Sig-Status", middleware.SignatureStatusFromContext(r))
				w.WriteHeader(http.StatusAccepted)
			}),
		),
	)
}

func (s *HmacSignatureSuite) TestHMACSignature() {
	payload := []byte(`{"webhook_event_type":"order_approved"}`)
	scenarios := []struct {
		name               string
		path               string
		headerValue        string
		secretCurrent      string
		secretNext         string
		expectedHTTPStatus int
		expectedSigStatus  string
	}{
		{
			name:               "deve validar assinatura pela query string",
			path:               "/?signature=" + buildSignature(payload, testSecret),
			secretCurrent:      testSecret,
			expectedHTTPStatus: http.StatusAccepted,
			expectedSigStatus:  middleware.SignatureStatusValid,
		},
		{
			name:               "deve validar assinatura pelo header como fallback",
			path:               "/",
			headerValue:        buildSignature(payload, testSecret),
			secretCurrent:      testSecret,
			expectedHTTPStatus: http.StatusAccepted,
			expectedSigStatus:  middleware.SignatureStatusValid,
		},
		{
			name:               "deve priorizar a assinatura da query sobre o header",
			path:               "/?signature=" + buildSignature(payload, testSecret),
			headerValue:        "wronginheader",
			secretCurrent:      testSecret,
			expectedHTTPStatus: http.StatusAccepted,
			expectedSigStatus:  middleware.SignatureStatusValid,
		},
		{
			name:               "deve rejeitar assinatura invalida no middleware",
			path:               "/?signature=invalidhex",
			secretCurrent:      testSecret,
			expectedHTTPStatus: http.StatusUnauthorized,
			expectedSigStatus:  "",
		},
		{
			name:               "deve aceitar segredo rotacionado",
			path:               "/?signature=" + buildSignature(payload, testSecretNext),
			secretCurrent:      testSecret,
			secretNext:         testSecretNext,
			expectedHTTPStatus: http.StatusAccepted,
			expectedSigStatus:  middleware.SignatureStatusRotated,
		},
		{
			name: "deve rejeitar assinatura alterada no middleware",
			path: func() string {
				realSig := buildSignature(payload, testSecret)
				tampered := realSig[:len(realSig)-1] + "0"
				if tampered == realSig {
					tampered = realSig[:len(realSig)-1] + "1"
				}
				return "/?signature=" + tampered
			}(),
			secretCurrent:      testSecret,
			expectedHTTPStatus: http.StatusUnauthorized,
			expectedSigStatus:  "",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			handler := newHandler(scenario.secretCurrent, scenario.secretNext)
			req := httptest.NewRequest(http.MethodPost, scenario.path, strings.NewReader(string(payload)))
			if scenario.headerValue != "" {
				req.Header.Set("X-Kiwify-Signature", scenario.headerValue)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(s.T(), scenario.expectedHTTPStatus, rr.Code)
			if scenario.expectedSigStatus != "" {
				assert.Equal(s.T(), scenario.expectedSigStatus, rr.Header().Get("X-Sig-Status"))
			}
		})
	}
}

func (s *HmacSignatureSuite) TestHMACSignature_RealKiwifyVectors() {
	const secret = "9ch0bpzogu9"

	vectors := []struct {
		name string
		sig  string
		body []byte
	}{
		{
			name: "order_approved compra real cartao",
			sig:  "e8c9bfc3080b49d11d026058171c9061bc5cde95",
			body: []byte(`{"order_id":"aac95806-d613-4cc6-80f9-f74882dbbce2","order_ref":"cbQHbMQ","order_status":"paid","product_type":"payment","payment_method":"credit_card","store_id":"IUuICjDIMZh18ff","payment_merchant_id":"10492606081149378630","installments":1,"card_type":"visa","card_last4digits":"3242","boleto_barcode":null,"boleto_expiry_date":null,"pix_code":null,"pix_expiration":null,"sale_type":"producer","created_at":"2026-06-08 11:53","updated_at":"2026-06-08 11:53","approved_date":"2026-06-08 11:53","refunded_at":null,"webhook_event_type":"order_approved","Product":{"product_id":"7b9afcf0-6049-11f1-9238-450795acb5a2","product_name":"MeControla - Seu agente financeiro direto no WhatsApp"},"Customer":{"full_name":"Jailton Angelo Teixeira Junior","first_name":"Jailton","email":"jailton.junior94@outlook.com","mobile":"+5511986896322","CPF":"43040606867","ip":"138.97.243.135","country":"br"},"Commissions":{"charge_amount":500,"product_base_price":500,"product_base_price_currency":"BRL","kiwify_fee":294,"kiwify_fee_currency":"BRL","commissioned_stores":[{"id":"a37d70a0-5675-4e3a-aa55-ebcf3b2868d0","type":"producer","custom_name":"Stefany Kelly Lima Teixeira","email":"stefanykelly.lima@hotmail.com","value":"206"}],"currency":"BRL","my_commission":206,"funds_status":"waiting","estimated_deposit_date":"2026-06-23T03:00:00.000Z","deposit_date":null},"TrackingParameters":{"src":null,"sck":"01hmtkn-fake-uuid-aaa","utm_source":null,"utm_medium":null,"utm_campaign":null,"utm_content":null,"utm_term":null,"s1":null,"s2":null,"s3":null},"checkout_link":"LuSveVz","Subscription":{"start_date":"2026-06-08T14:53:19.679Z","next_payment":"2026-07-08T14:53:23.137Z","status":"active","customer_access":{"has_access":true,"active_period":true,"access_until":"2026-07-08T14:53:23.137Z"},"plan":{"id":"97cd6da6-c095-426d-b3d0-0efeeaea0042","name":"Teste2","frequency":"monthly","qty_charges":0},"charges":{"completed":[{"order_id":"aac95806-d613-4cc6-80f9-f74882dbbce2","amount":206,"status":"paid","installments":1,"card_type":"visa","card_last_digits":"3242","card_first_digits":"499818","created_at":"2026-06-08T14:53:19.981Z"}],"future":[{"charge_date":"2026-07-08T14:53:23.137Z"},{"charge_date":"2026-08-08T14:53:23.137Z"},{"charge_date":"2026-09-08T14:53:23.137Z"},{"charge_date":"2026-10-08T14:53:23.137Z"},{"charge_date":"2026-11-08T14:53:23.137Z"},{"charge_date":"2026-12-08T14:53:23.137Z"},{"charge_date":"2027-01-08T14:53:23.137Z"},{"charge_date":"2027-02-08T14:53:23.137Z"},{"charge_date":"2027-03-08T14:53:23.137Z"},{"charge_date":"2027-04-08T14:53:23.137Z"},{"charge_date":"2027-05-08T14:53:23.137Z"}]}},"subscription_id":"9584c28e-8c7b-44bc-9282-2fa03c45b7db","access_url":null}`),
		},
		{
			name: "billet_created boleto gerado",
			sig:  "06f61a6aff46cea884f02a981caedc92c3c247b8",
			body: []byte(`{"order_id":"ce936450-f853-4f66-9c72-03964fa436ea","order_ref":"DxfWDlN","order_status":"waiting_payment","product_type":"membership","payment_method":"boleto","store_id":"yOqrzDK1JFUIPlX","payment_merchant_id":19431960,"installments":null,"card_type":"","card_last4digits":"","card_rejection_reason":null,"boleto_URL":"https://pagar.me","boleto_barcode":"23791.22928 60005.510098 74000.046909 4 83110000300000","boleto_expiry_date":"12/06/2026","pix_code":null,"pix_expiration":null,"sale_type":"producer","created_at":"2026-06-08 15:42","updated_at":"2026-06-08 15:42","approved_date":null,"refunded_at":null,"webhook_event_type":"billet_created","Product":{"product_id":"d295c1f8-26ff-47de-9c72-bf73e6945cce","product_name":"Example product"},"Customer":{"full_name":"John Doe","first_name":"John","email":"johndoe@example.com","mobile":"+41945968750","CPF":"45658495664","ip":"82.253.125.70","instagram":"@kiwify","street":"Rua 1001","number":"315","complement":"SL 05","neighborhood":"Centro","city":"Balneário Camboriú","state":"SC","zipcode":"88330-756"},"Commissions":{"charge_amount":1840,"product_base_price":1840,"product_base_price_currency":"BRL","kiwify_fee":202,"kiwify_fee_currency":"BRL","settlement_amount":1840,"settlement_amount_currency":"BRL","sale_tax_rate":0,"sale_tax_amount":0,"commissioned_stores":[{"id":"e79c4e6e-f7ae-4d8a-9cf3-4905928943d6","type":"producer","custom_name":"Example store","email":"example@store.domain","value":"1638"},{"id":"1ee2f7c3-22bc-44eb-96d7-cf03d62a80ac","type":"coproducer","custom_name":"Example coproducer","email":"example@coproducer.domain","value":"1638"},{"id":"77a5eed3-d24f-437b-8c15-618dcd878cfb","type":"affiliate","affiliate_id":"jJXoElY","custom_name":"Example affiliate","email":"example@affiliate.domain","value":"1638"}],"currency":"BRL","my_commission":1638,"funds_status":null,"estimated_deposit_date":null,"deposit_date":null},"TrackingParameters":{"src":null,"sck":null,"utm_source":null,"utm_medium":null,"utm_campaign":null,"utm_content":null,"utm_term":null,"s1":null,"s2":null,"s3":null}}`),
		},
		{
			name: "pix_created pix gerado com subscription",
			sig:  "c917b9ac6d93c7f7a4ae2163aff0bdc482d1cc75",
			body: []byte(`{"order_id":"d233a1b5-aa46-4fc4-bae9-8058d2c85634","order_ref":"phDwpkZ","order_status":"waiting_payment","product_type":"membership","payment_method":"pix","store_id":"WedLlRnF5LFMWiW","payment_merchant_id":71293865,"installments":null,"card_type":"","card_last4digits":"","card_rejection_reason":null,"boleto_URL":null,"boleto_barcode":null,"boleto_expiry_date":null,"pix_code":"00020101021226840014br.gov.bcb.pix2562pix-h.stone.com.br/pix/v2/ee77b9e9-67fb-4b62-829b-b6537068aa7b5204000053039865406970.005802BR5924Pagar.me Pagamentos S.A.6014RIO DE JANEIRO622905253b6997c1ef2f4b2597b6e11de6304ED77","pix_expiration":"12/06/2026 15:42","sale_type":"producer","created_at":"2026-06-08 15:42","updated_at":"2026-06-08 15:42","approved_date":null,"refunded_at":null,"webhook_event_type":"pix_created","Product":{"product_id":"61628579-9468-4bee-996e-6dcf353270c1","product_name":"Example product"},"Customer":{"full_name":"John Doe","first_name":"John","email":"johndoe@example.com","mobile":"+92588043149","cnpj":"21907570836100","ip":"8a63:c91f:9aa1:cb10:6ef:471e:548e:47fd","instagram":"@kiwify","street":"Rua 1001","number":"315","complement":"SL 05","neighborhood":"Centro","city":"Balneário Camboriú","state":"SC","zipcode":"88330-756"},"Commissions":{"charge_amount":6772,"product_base_price":6772,"product_base_price_currency":"BRL","kiwify_fee":745,"kiwify_fee_currency":"BRL","settlement_amount":6772,"settlement_amount_currency":"BRL","sale_tax_rate":0,"sale_tax_amount":0,"commissioned_stores":[{"id":"3851a9a5-8147-4593-b1f9-8d6f2b93ebf3","type":"producer","custom_name":"Example store","email":"example@store.domain","value":"6027"},{"id":"ca2ec145-c93d-41c1-bd80-276e3766633c","type":"coproducer","custom_name":"Example coproducer","email":"example@coproducer.domain","value":"6027"},{"id":"de19ab7d-d2dc-4c2f-89ad-772ac2e4ae61","type":"affiliate","affiliate_id":"vpqyrRj","custom_name":"Example affiliate","email":"example@affiliate.domain","value":"6027"}],"currency":"BRL","my_commission":6027,"funds_status":null,"estimated_deposit_date":null,"deposit_date":null},"TrackingParameters":{"src":null,"sck":null,"utm_source":null,"utm_medium":null,"utm_campaign":null,"utm_content":null,"utm_term":null,"s1":null,"s2":null,"s3":null},"Subscription":{"id":"aa136b55-4056-401f-a8d1-83664d9a35a7","start_date":"2026-06-05T15:42:55.415Z","next_payment":"2026-06-12T15:42:55.415Z","status":"active","plan":{"id":"ec2d9d36-23f2-41d0-a8de-b6e794d2a3db","name":"Example plan","frequency":"weekly","qty_charges":0},"charges":{"completed":[{"order_id":"d233a1b5-aa46-4fc4-bae9-8058d2c85634","amount":6027,"status":"paid","installments":1,"card_type":"mastercard","card_last_digits":"7178","card_first_digits":"902315","created_at":"2026-06-05T15:42:55.415Z"}],"future":[{"charge_date":"2026-06-12T15:42:55.415Z"}]}},"subscription_id":"aa136b55-4056-401f-a8d1-83664d9a35a7"}`),
		},
	}

	for _, v := range vectors {
		s.Run(v.name, func() {
			assert.Equal(s.T(), v.sig, buildSignature(v.body, secret))

			req := httptest.NewRequest(http.MethodPost, "/?signature="+v.sig, strings.NewReader(string(v.body)))
			rr := httptest.NewRecorder()
			newHandler(secret, "").ServeHTTP(rr, req)

			assert.Equal(s.T(), http.StatusAccepted, rr.Code)
			assert.Equal(s.T(), middleware.SignatureStatusValid, rr.Header().Get("X-Sig-Status"))
		})
	}
}
