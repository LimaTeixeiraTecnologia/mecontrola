# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Janela de ativação de 24h medida a partir de `paidAt`
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Plataforma / autor da feature
- **Relacionados:** PRD `.specs/prd-ativacao-whatsapp/prd.md` (RF-10, RF-23), techspec `.specs/prd-ativacao-whatsapp/techspec.md`, `.agents/skills/go-implementation/references/domain-modeling.md`

## Contexto

O PRD de origem pede "TTL de 10 minutos" para a Activation Session. Investigação do código revelou que o `magic_token` é criado no **checkout** (`create_checkout_session.go:74`), antes do pagamento, com `expiresAt = createdAt + TokenTTLDays` (default 7 dias). Medir 10 minutos a partir do checkout é inviável: o token expiraria antes do pagamento. A intenção real do PRD é uma **janela curta de ativação após o pagamento**.

`expiresAt` também é usado pelo job `expire_tokens.go` (`BulkExpire`) e por `get_token_state`/`consume_magic_token` via `IsExpiredAt(now)`. Alterar sua semântica quebraria o ciclo de vida existente do token de checkout.

## Decisão

A **janela de ativação** é de **24 horas a partir de `paidAt`** (configurável via `ONBOARDING_ACTIVATION_WINDOW_HOURS`, default 24), implementada **sem alterar `expiresAt`**:

1. Guard de domínio puro novo (DMMF Princípio 6, sem ctx/IO):
   `MagicToken.IsActivationWindowOpen(now, window) bool` = `status==PAID && !paidAt.IsZero() && now.Sub(paidAt) ∈ [0, window]`.
2. Query dedicada `FindActivableByMobile(ctx, mobileE164, paidAfter)` com `paid_at > $2` (`$2 = now - window`) e `ORDER BY paid_at DESC LIMIT 1` (também satisfaz RF-23, recompra → mais recente).
3. `expiresAt` (checkout) e o job de expiração permanecem intactos como salvaguarda de vida longa do token.
4. A janela vale nos **dois caminhos de correlação**: telefone (via `FindActivableByMobile`) e token (`ConsumeMagicToken` passa a exigir `IsActivationWindowOpen` além de `!IsExpiredAt`), garantindo semântica única de ativação.

O valor 24h (em vez dos 10 min literais) foi decidido com o usuário para evitar expiração em massa de usuários legítimos que demoram a abrir o e-mail.

## Alternativas Consideradas

- **10 min a partir de `paidAt`**: fiel ao número do PRD, − alta taxa de expiração legítima (e-mail aberto tarde) → muito fallback/suporte. Rejeitada (decisão explícita do usuário).
- **10 min a partir do checkout**: inviável (token nasce antes do pagamento). Rejeitada.
- **Reaproveitar `expiresAt` redefinindo-o em `MarkPaid`**: − quebraria a semântica do token de checkout e o job de expiração; − acoplaria dois conceitos (vida do token × janela de ativação). Rejeitada.

## Consequências

### Benefícios Esperados

- Janela curta pós-pagamento sem quebrar o ciclo de vida existente nem o job de expiração.
- Guard puro e query testáveis isoladamente; sem abstração de tempo.
- Seleção determinística do token mais recente cobre recompra/renovação.

### Trade-offs e Custos

- Dois conceitos temporais coexistem (`expiresAt` de checkout × janela de `paidAt`); documentar para evitar confusão.
- Índice parcial novo por telefone.

### Riscos e Mitigações

- **`paidAt` zero/ausente** (token nunca marcado PAID): guard retorna falso e a query filtra por `status='PAID'`, então não ativa — correto.
- **Relógio/skew**: janela tolerante a `now.Before(paidAt)` (retorna fechado), evitando ativação com `paidAt` futuro.
- Rollback: remover guard/query e ajustar config; sem dado destrutivo (índice é aditivo).

## Plano de Implementação

1. Config `ONBOARDING_ACTIVATION_WINDOW_HOURS` (default 24) + default loader.
2. Guard `IsActivationWindowOpen` + unit tests de borda.
3. Query `FindActivableByMobile` + índice parcial + integration test (janela, recompra).
4. Consumir o guard/janela no usecase `ActivateFromInbound`.
5. Conclusão: integração comprova ativação dentro da janela e bloqueio fora dela.

## Monitoramento e Validação

- `onboarding_activation_window_expired_total` para tentativas fora da janela.
- Acompanhar proporção `no_match`/`window_expired` vs `phone_matched`; se expiração legítima subir, reavaliar o valor da janela.

## Impacto em Documentação e Operação

- `.env.example` e runbook: nova variável e a distinção `expiresAt` × janela de ativação.

## Revisão Futura

Reavaliar o valor de 24h após dados reais de abertura de e-mail; revisitar se o checkout passar a criar token com telefone conhecido (mudaria a correlação).
