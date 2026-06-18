//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerCardSteps(sc *godog.ScenarioContext, e *e2eCtx) {
	sc.Step(`^existe um usuário autenticado$`, e.existeUmUsuarioAutenticado)
	sc.Step(`^o usuário cria um cartão "([^"]*)" com limite de R\$ (\d+),(\d{2}), fechamento (\d+) e vencimento (\d+)$`, e.oUsuarioCriaUmCartao)
	sc.Step(`^o cartão deve estar salvo no banco com limite de R\$ (\d+),(\d{2})$`, e.oCartaoDeveEstarSalvoNoBanco)
	sc.Step(`^o usuário já possui um cartão com limite de R\$ (\d+),(\d{2})$`, e.oUsuarioJaPossuiUmCartao)
	sc.Step(`^o usuário solicita o aumento para R\$ (\d+),(\d{2})$`, e.oUsuarioSolicitaOAumento)
	sc.Step(`^a leitura do cartão deve refletir o limite de R\$ (\d+),(\d{2})$`, e.aLeituraDoCartaoDeveRefletirOLimite)
	sc.Step(`^o usuário possui um cartão com fatura vencendo em (\d+) dias$`, e.oUsuarioPossuiUmCartaoComFaturaVencendoEmDias)
	sc.Step(`^o worker de alertas de fatura é executado$`, e.oWorkerDeAlertasDeFaturaEExecutado)
	sc.Step(`^o worker de alertas de fatura é executado novamente$`, e.oWorkerDeAlertasDeFaturaEExecutadoNovamente)
	sc.Step(`^deve existir (\d+) evento do tipo "([^"]*)" no outbox para o cartão$`, e.deveExistirEventoDoTipoNoOutboxParaOCartao)
	sc.Step(`^o payload do evento deve referenciar o cartão e o vencimento em (\d+) dias$`, e.oPayloadDoEventoDeveReferenciarOCartao)
	sc.Step(`^deve existir apenas (\d+) registro de alerta para o cartão$`, e.deveExistirApenasRegistroDeAlertaParaOCartao)
}

func (e *e2eCtx) existeUmUsuarioAutenticado() error {
	return nil
}

func (e *e2eCtx) oUsuarioCriaUmCartao(nome string, reais, centavos, fechamento, vencimento int) error {
	return e.createCardViaHTTP(nome, fechamento, vencimento, brlToCents(reais, centavos))
}

func (e *e2eCtx) oCartaoDeveEstarSalvoNoBanco(reais, centavos int) error {
	if e.cardID == "" {
		return fmt.Errorf("ID do cartão não capturado")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		cardID     string
		userID     string
		name       string
		limitCents int64
	)
	err := e.mgr.DBTX(ctx).QueryRowContext(ctx, `
		SELECT id, user_id, name, limit_cents
		  FROM mecontrola.cards
		 WHERE id = $1
	`, e.cardID).Scan(&cardID, &userID, &name, &limitCents)
	if err != nil {
		return fmt.Errorf("consultar cartão no banco: %w", err)
	}

	if cardID != e.cardID {
		return fmt.Errorf("cartão persistido divergente: %s", cardID)
	}
	if userID != e.userID.String() {
		return fmt.Errorf("user_id esperado %s, recebido %s", e.userID.String(), userID)
	}
	if name != e.cardName {
		return fmt.Errorf("nome esperado %q, recebido %q", e.cardName, name)
	}
	expected := brlToCents(reais, centavos)
	if limitCents != expected {
		return fmt.Errorf("limit_cents esperado %d, recebido %d", expected, limitCents)
	}
	return nil
}

func (e *e2eCtx) oUsuarioJaPossuiUmCartao(reais, centavos int) error {
	return e.createCardViaHTTP(e.uniqueCardName("Limite"), 5, 12, brlToCents(reais, centavos))
}

func (e *e2eCtx) oUsuarioSolicitaOAumento(reais, centavos int) error {
	if e.cardID == "" {
		return fmt.Errorf("nenhum cartão disponível para atualização")
	}

	payload := map[string]any{
		"limit_cents": brlToCents(reais, centavos),
	}
	return e.makeRequest(http.MethodPatch, "/api/v1/cards/"+e.cardID+"/limit", payload)
}

func (e *e2eCtx) aLeituraDoCartaoDeveRefletirOLimite(reais, centavos int) error {
	if e.cardID == "" {
		return fmt.Errorf("nenhum cartão disponível para leitura")
	}
	if err := e.makeRequest(http.MethodGet, "/api/v1/cards/"+e.cardID+"/", nil); err != nil {
		return err
	}
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}
	if e.lastResp.StatusCode != http.StatusOK {
		return fmt.Errorf("status esperado 200, recebido %d, corpo: %s", e.lastResp.StatusCode, e.lastBodyText)
	}

	rawLimit, ok := e.lastBody["limit_cents"]
	if !ok {
		return fmt.Errorf("campo limit_cents ausente no corpo")
	}
	expected := brlToCents(reais, centavos)
	got, ok := rawLimit.(float64)
	if !ok {
		return fmt.Errorf("campo limit_cents com tipo inesperado %T", rawLimit)
	}
	if int64(got) != expected {
		return fmt.Errorf("limit_cents esperado %d, recebido %d", expected, int64(got))
	}
	return nil
}

func (e *e2eCtx) oUsuarioPossuiUmCartaoComFaturaVencendoEmDias(days int) error {
	dueDate := time.Now().UTC().AddDate(0, 0, days)
	closingDay := dueDate.AddDate(0, 0, -5).Day()
	if closingDay < 1 {
		closingDay = 1
	}
	if err := e.createCardViaHTTP(e.uniqueCardName("Fatura"), closingDay, dueDate.Day(), 500000); err != nil {
		return err
	}
	e.expectedDueDate = time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, time.UTC)
	e.expectedDaysUntil = days
	return nil
}

func (e *e2eCtx) oWorkerDeAlertasDeFaturaEExecutado() error {
	return e.runInvoiceDueAlertsJob()
}

func (e *e2eCtx) oWorkerDeAlertasDeFaturaEExecutadoNovamente() error {
	return e.runInvoiceDueAlertsJob()
}

func (e *e2eCtx) deveExistirEventoDoTipoNoOutboxParaOCartao(expected int, eventType string) error {
	if e.cardID == "" {
		return fmt.Errorf("nenhum cartão disponível para consulta do outbox")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	err := e.mgr.DBTX(ctx).QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM mecontrola.outbox_events
		 WHERE event_type = $1
		   AND aggregate_id = $2
		   AND aggregate_user_id = $3
	`, eventType, e.cardID, e.userID.String()).Scan(&total)
	if err != nil {
		return fmt.Errorf("consultar outbox: %w", err)
	}
	if total != expected {
		return fmt.Errorf("quantidade de eventos esperada %d, recebida %d", expected, total)
	}
	return nil
}

func (e *e2eCtx) oPayloadDoEventoDeveReferenciarOCartao(days int) error {
	if e.cardID == "" {
		return fmt.Errorf("nenhum cartão disponível para validar payload")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var payload []byte
	err := e.mgr.DBTX(ctx).QueryRowContext(ctx, `
		SELECT payload
		  FROM mecontrola.outbox_events
		 WHERE event_type = 'card.invoice_due.v1'
		   AND aggregate_id = $1
		 ORDER BY occurred_at DESC
		 LIMIT 1
	`, e.cardID).Scan(&payload)
	if err != nil {
		return fmt.Errorf("consultar payload do outbox: %w", err)
	}

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return fmt.Errorf("decodificar payload do outbox: %w", err)
	}

	if body["card_id"] != e.cardID {
		return fmt.Errorf("payload card_id esperado %s, recebido %v", e.cardID, body["card_id"])
	}
	if body["user_id"] != e.userID.String() {
		return fmt.Errorf("payload user_id esperado %s, recebido %v", e.userID.String(), body["user_id"])
	}
	if body["card_name"] != e.cardName {
		return fmt.Errorf("payload card_name esperado %q, recebido %v", e.cardName, body["card_name"])
	}
	if body["due_date"] != e.expectedDueDate.Format("2006-01-02") {
		return fmt.Errorf("payload due_date esperado %s, recebido %v", e.expectedDueDate.Format("2006-01-02"), body["due_date"])
	}
	rawDays, ok := body["days_until"].(float64)
	if !ok {
		return fmt.Errorf("payload days_until com tipo inesperado %T", body["days_until"])
	}
	if int(rawDays) != days || int(rawDays) != e.expectedDaysUntil {
		return fmt.Errorf("payload days_until esperado %d, recebido %d", days, int(rawDays))
	}
	return nil
}

func (e *e2eCtx) deveExistirApenasRegistroDeAlertaParaOCartao(expected int) error {
	if e.cardID == "" {
		return fmt.Errorf("nenhum cartão disponível para consulta do ledger")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	err := e.mgr.DBTX(ctx).QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM mecontrola.card_invoice_alerts_sent
		 WHERE user_id = $1
		   AND card_id = $2
	`, e.userID.String(), e.cardID).Scan(&total)
	if err != nil {
		return fmt.Errorf("consultar ledger de alertas: %w", err)
	}
	if total != expected {
		return fmt.Errorf("quantidade de registros esperada %d, recebida %d", expected, total)
	}
	return nil
}

func (e *e2eCtx) createCardViaHTTP(name string, closingDay, dueDay int, limitCents int64) error {
	payload := map[string]any{
		"name":        name,
		"nickname":    e.uniqueNickname(name),
		"closing_day": closingDay,
		"due_day":     dueDay,
		"limit_cents": limitCents,
	}
	if err := e.makeRequest(http.MethodPost, "/api/v1/cards/", payload); err != nil {
		return err
	}
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}
	if e.lastResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("status esperado 201, recebido %d, corpo: %s", e.lastResp.StatusCode, e.lastBodyText)
	}
	id, ok := e.lastBody["id"].(string)
	if !ok || id == "" {
		return fmt.Errorf("resposta de criação sem id válido: %s", e.lastBodyText)
	}
	e.cardID = id
	e.cardName = name
	return nil
}

func (e *e2eCtx) runInvoiceDueAlertsJob() error {
	if e.invoiceDueAlertsJob == nil {
		return fmt.Errorf("job de alertas de fatura não disponível no suite e2e")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return e.invoiceDueAlertsJob.Run(ctx)
}

func (e *e2eCtx) uniqueCardName(prefix string) string {
	return fmt.Sprintf("%s %d", prefix, time.Now().UnixNano())
}

func (e *e2eCtx) uniqueNickname(prefix string) string {
	base := strings.ToLower(strings.ReplaceAll(prefix, " ", "-"))
	if len(base) > 20 {
		base = base[:20]
	}
	return fmt.Sprintf("%s-%s", base, uuid.NewString()[:8])
}

func brlToCents(reais, centavos int) int64 {
	return int64(reais*100 + centavos)
}
