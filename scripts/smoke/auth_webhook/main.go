package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const smokeUserID = "00000000-0000-0000-0000-00005a17c8e7"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "smoke: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("smoke: PASS")
}

func run() error {
	webhookURL := flag.String("url", os.Getenv("WEBHOOK_URL"), "URL do webhook (ex: https://staging.example.com/api/v1/whatsapp/inbound)") //nolint:forbidigo // script CLI standalone fora do domínio
	secret := flag.String("secret", os.Getenv("META_APP_SECRET"), "META_APP_SECRET para HMAC-SHA256")                                      //nolint:forbidigo
	userWA := flag.String("user-wa", os.Getenv("SMOKE_WA"), "Numero WhatsApp do usuario de smoke (E.164)")                                 //nolint:forbidigo
	dbURL := flag.String("db", os.Getenv("DB_URL"), "URL de conexao Postgres (opcional — valida linha em auth_events)")                    //nolint:forbidigo
	flag.Parse()

	if *webhookURL == "" {
		return errors.New("--url ou WEBHOOK_URL e obrigatorio")
	}
	if *secret == "" {
		return errors.New("--secret ou META_APP_SECRET e obrigatorio")
	}
	if *userWA == "" {
		return errors.New("--user-wa ou SMOKE_WA e obrigatorio")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wamid := fmt.Sprintf("wamid.smoke.%d", time.Now().UnixNano())
	body, err := buildPayload(*userWA, wamid)
	if err != nil {
		return fmt.Errorf("build payload: %w", err)
	}

	sig := computeHMAC(*secret, body)

	if err := postWebhook(ctx, *webhookURL, sig, body); err != nil {
		return fmt.Errorf("webhook POST: %w", err)
	}

	fmt.Println("smoke: webhook POST OK (HTTP 200)")

	if *dbURL != "" {
		if err := assertAuthEvent(ctx, *dbURL); err != nil {
			return fmt.Errorf("sql assertion: %w", err)
		}
		fmt.Println("smoke: auth_events assertion OK")
	}

	return nil
}

type waPayload struct {
	Object string    `json:"object"`
	Entry  []waEntry `json:"entry"`
}

type waEntry struct {
	ID      string     `json:"id"`
	Changes []waChange `json:"changes"`
}

type waChange struct {
	Field string      `json:"field"`
	Value waChangeVal `json:"value"`
}

type waChangeVal struct {
	MessagingProduct string      `json:"messaging_product"`
	Messages         []waMessage `json:"messages"`
}

type waMessage struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      waText `json:"text"`
}

type waText struct {
	Body string `json:"body"`
}

func buildPayload(from, wamid string) ([]byte, error) {
	p := waPayload{
		Object: "whatsapp_business_account",
		Entry: []waEntry{
			{
				ID: "smoke-entry",
				Changes: []waChange{
					{
						Field: "messages",
						Value: waChangeVal{
							MessagingProduct: "whatsapp",
							Messages: []waMessage{
								{
									From:      from,
									ID:        wamid,
									Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
									Type:      "text",
									Text:      waText{Body: "smoke test"},
								},
							},
						},
					},
				},
			},
		},
	}
	return json.Marshal(p)
}

func computeHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func postWebhook(ctx context.Context, url, sig string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("esperado HTTP 200, obteve %d", resp.StatusCode)
	}
	return nil
}

func assertAuthEvent(ctx context.Context, dbURL string) error {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	cutoff := time.Now().UTC().Add(-10 * time.Second)
	var count int
	err = db.QueryRowContext(ctx,
		`SELECT count(*) FROM auth_events WHERE user_id = $1 AND occurred_at > $2`,
		smokeUserID,
		cutoff,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("query auth_events: %w", err)
	}
	if count == 0 {
		return errors.New("nenhuma linha em auth_events para o usuario de smoke nos ultimos 10 segundos")
	}
	return nil
}
