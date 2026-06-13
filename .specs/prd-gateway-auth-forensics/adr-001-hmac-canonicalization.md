# ADR-001 — Canonicalização HMAC do Gateway

## Metadados

- **Título:** Canonical message do HMAC-SHA256 do gateway LLM ↔ API
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola (Jailton Junior); revisado em multiple-choice protocol
- **Relacionados:** [PRD](prd.md) RF-04, RF-07; [techspec](techspec.md) seção "Design de Implementação"; plano-fonte `docs/planos/2026-06-11-auditoria-seguranca-pre-golive.md` B1

## Contexto

O middleware `RequireGatewayAuth` precisa calcular HMAC sobre uma representação canonical da requisição. Sem canonicalização cravada antes do código, dois implementadores (servidor Go e cliente LLM) produzem schemas incompatíveis e a verificação sempre falha — ou pior, passa por coincidência durante teste e falha em produção.

Restrições:
- Cliente LLM provavelmente em Python; canonical precisa ser reproduzível trivialmente em qualquer linguagem.
- App Go é o consumidor final; canonical precisa ser barata de calcular (< 50µs).
- Sem cache de nonce no MVP (ADR-003); canonical não pode ser o único mecanismo contra replay.

## Decisão

Canonical message = `strings.ToLower(userIDRaw) + "." + timestampRaw`.

- `userIDRaw`: string do header `X-User-ID` aplicada `ToLower` (uuids canônicos são lowercase; tolera variação do cliente).
- `timestampRaw`: string do header `X-Gateway-Timestamp` **como recebida** (sem reparseio para canonical). Cliente e servidor mantêm exatamente os mesmos bytes.
- Separador `.` (ponto): escolhido por não aparecer em UUID nem em unix epoch decimal.

HMAC = `hex(hmac_sha256(secret, canonical))` em lowercase. Validação no servidor com `hmac.Equal` (constant-time).

**Não inclui**: método HTTP, path, hash do body, query string. Decisão consciente para MVP.

## Alternativas Consideradas

1. **Incluir method + path** — protege contra reuso da assinatura em endpoint diferente. **Rejeitada**: cliente LLM precisa conhecer path final após rewrites do Caddy; risco de mismatch silencioso. Benefício pequeno (todas as rotas com gateway hoje são `/api/v1/cards*`).
2. **Incluir sha256(body)** — protege contra replay de payload exato. **Rejeitada para MVP**: cliente precisa hashear body antes de enviar; servidor precisa bufferizar body (custo extra de memória). Idempotency-Key já protege mutations. Reconsiderar pós go-live se um incidente real mostrar replay de payload.
3. **JWT em vez de HMAC** — entrega claims tipadas e expiração nativa. **Rejeitada**: introduz dependência (`golang-jwt`), requer JWKS para rotação, e o escopo desta auditoria é MVP. Migração para JWT está documentada como pós go-live no plano-fonte seção 6.
4. **Base64 em vez de hex** — economia de 25% no tamanho do header. **Rejeitada**: hex lowercase é mais auditável em logs (humano lê), e o ganho é insignificante (~32 bytes a menos).

## Consequências

### Benefícios Esperados

- Reproduzibilidade trivial em qualquer linguagem (concat + HMAC + hex).
- Overhead mínimo: < 5µs no Go para HMAC + 1 alloc transitória.
- Vetor de teste fixo (`user=00000000-...`, `ts=1700000000`) reproduzível em Python para sanidade cross-lang.

### Trade-offs e Custos

- Assinatura é replayable dentro de 60s para qualquer endpoint do mesmo `user_id`.
- Assinatura é replayable para qualquer body do mesmo `user_id`.
- Mutações ficam protegidas apenas por `Idempotency-Key` (já existente).

### Riscos e Mitigações

- **R-01**: ataque on-path captura header + replica em endpoint diferente. **Mitigação**: TLS termina no Caddy; rede interna controlada; Idempotency-Key impede mutation duplicada.
- **R-02**: cliente LLM canoniza errado (e.g. timestamp reparseado vs raw). **Mitigação**: vetor de teste fixo publicado no runbook + teste unitário Go com vetor idêntico.

## Plano de Implementação

1. Smart constructor `NewGatewaySignature(hex string)` valida charset hex lowercase + len=64.
2. Função pura `canonical(userIDRaw, timestampRaw string) string` em `domain/services/`.
3. Teste com vetor fixo (input + expected hex) em `verify_gateway_request_test.go`.
4. Documentar vetor fixo no runbook `docs/runbooks/gateway-auth.md` para handoff ao operador da LLM.

## Monitoramento e Validação

- Métrica `identity_gateway_auth_total{result="invalid_signature"}` em estado estacionário deve ser ≈ 0. Pico esporádico tolerado durante rotação.
- Alerta operacional se taxa > 1% por 10 min.

## Impacto em Documentação e Operação

- `docs/runbooks/gateway-auth.md`: descrição da canonical + vetor fixo + exemplo Python.
- Cliente LLM (fora deste repo): código de assinatura sincronizado com vetor fixo.

## Revisão Futura

Revisar quando:
- Houver um segundo endpoint com semântica de mutação diferente onde body hash agregue valor.
- Houver app móvel/web direto (sem LLM intermediária) — provável migração para JWT.
- Data sugerida: 2027-06-12.
