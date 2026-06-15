package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	url := flag.String("url", "http://localhost:8080/api/v1/channels/telegram/webhook", "Telegram webhook URL")
	secret := flag.String("secret", "", "Secret token (X-Telegram-Bot-Api-Secret-Token header)")
	fromID := flag.Int64("from-id", 987654321, "Telegram from.id (int64)")
	text := flag.String("text", "Quanto gastei esse mes?", "Message text")
	updateID := flag.Int64("update-id", time.Now().Unix(), "update_id")
	missingHeader := flag.Bool("missing-header", false, "Omit secret header (should get 401)")
	flag.Parse()

	if *secret == "" && !*missingHeader {
		log.Fatal("--secret required (or set TELEGRAM_SECRET_TOKEN env)")
	}

	body := fmt.Sprintf(`{
		"update_id": %d,
		"message": {
			"message_id": 1,
			"from": {"id": %d, "is_bot": false, "language_code": "pt-BR"},
			"chat": {"id": %d, "type": "private"},
			"date": %d,
			"text": %q
		}
	}`, *updateID, *fromID, *fromID, time.Now().Unix(), *text)

	req, err := http.NewRequest(http.MethodPost, *url, bytes.NewReader([]byte(body)))
	if err != nil {
		log.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if !*missingHeader {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", *secret)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("send: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("close body: %v", closeErr)
		}
	}()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	fmt.Printf("status: %d\n", resp.StatusCode)
	fmt.Printf("body: %s\n", string(respBody))

	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Println("OK — webhook autenticado e roteado")
	case http.StatusUnauthorized:
		if *missingHeader {
			fmt.Println("OK — sem header retorna 401 como esperado")
		} else {
			log.Fatal("FAIL — token rejeitado")
		}
	case http.StatusTooManyRequests:
		fmt.Println("rate limit hit (verificar TELEGRAM_WEBHOOK_RATE_LIMIT)")
	default:
		log.Fatalf("FAIL — status inesperado %d", resp.StatusCode)
	}
}
