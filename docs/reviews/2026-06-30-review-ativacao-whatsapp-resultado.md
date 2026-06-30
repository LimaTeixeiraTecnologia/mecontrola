# Resultado da Revisão — Jornada de Ativação via WhatsApp

- **Data:** 2026-06-30
- **Spec:** `.specs/prd-ativacao-whatsapp` (PRD RF-01..RF-37, techspec, 10 tasks, ADR-001/002/003)
- **Repos:** backend `/Users/jailtonjunior/Git/mecontrola` + landing `/Users/jailtonjunior/Git/mecontrola-landingpage`
- **Método:** skill `review` + 6 subagentes especializados (clusters) + reprodução executável + ciclo `review → bugfix → review`
- **Veredito final:** `APPROVED` (Rodada 2)

---

## Rodada 1 — `REJECTED`

### C1 — CRITICAL (RF-06/13/14) — e-mail de ativação nunca enviado
`internal/onboarding/infrastructure/email/templates/activation.html.tmpl:13` referenciava `{{.WaMeURL}}`,
campo inexistente em `ActivationTemplateInput` (`send_activation_email.go:17-21`). `html/template.Execute`
retorna `can't evaluate field WaMeURL` → `Render` falha → `SendActivationEmail.Execute` retorna erro antes
do `sender.Send` → outbox faz retry e vai a dead-letter. **Jornada bloqueada no passo 2** (paga → sem e-mail
→ nunca chega ao `/ativar`). Build e `go test` passavam porque o teste usa template stub.
- **Reprodução executável:** `template.Execute` contra struct sem o campo → `can't evaluate field WaMeURL`.
- **Confirmado por 2 subagentes independentes** (cluster e-mail + cluster webhook).

### Medianos
- **M1 (RF-32/33):** `configs/config.go:1221,1229` + `.env.example:204,214` — texto de boas-vindas/intro divergia
  das strings literais do PRD; **CTA textual "Vamos começar?" ausente** (intro terminava em "configuracao rapida").
- **M2 (RF-35/36):** `magic_token_repository.go:329-330` — `activation_started_at` gravado junto com `email_sent_at`
  (COALESCE set-once) → write real na ativação virava no-op; métrica "<30s" e reconstrução da jornada corrompidas.
- **M3 (DoD):** pacote `email/` sem teste que renderizasse o template real → C1 invisível à suíte.
- **M4 (RF-19):** landing `ativar.astro:9` (+ `dist`) — meta description "WhatsApp **ou Telegram**" visível em SEO/social.

### Baixos
- **L1 (RF-29):** `internal/platform/channels/activation_command.go` — `MatchActivationCommand` legado órfão.
- **L2 (RF-31):** descartado — token usa `base64.RawURLEncoding` (URL-safe); `url.QueryEscape` desnecessário (não é defeito).

---

## Remediação aplicada (causa raiz + regressão)

| Achado | Correção | Arquivo |
|--------|----------|---------|
| C1 | CTA → `{{.ActivationURL}}` (página `/ativar?token=`), rótulo "Ativar no WhatsApp" (RF-13) | `email/templates/activation.html.tmpl:13` |
| M3 | Teste que renderiza o template real e falharia contra `{{.WaMeURL}}` | `email/template_test.go` (novo) |
| M2 | `activation_started_at` removido de `UpdateSetEmailSentAt`; gravado só em `UpdateMarkActivationStartedAt` (ativação real) | `magic_token_repository.go:327-331` |
| M1 | Strings exatas do PRD (com 🎉, "Me Controla", "Seu WhatsApp agora está conectado", "Vamos começar?") | `configs/config.go:1221,1229`; `.env.example:204,214` |
| M4 | Meta description WhatsApp-only; removido `ativar-redirect.js` órfão e `PUBLIC_TELEGRAM_BOT`; `dist` rebuildado (0 telegram) | landing `ativar.astro:9`, `.env.example`, `public/js/` |
| L1 | Pacote `internal/platform/channels` removido (só referenciado pelo próprio teste) | — |

## M5 (RF-15) — risco residual que NÃO contradiz a spec
`email_sent_at` é gravado mas não relido como guard de reenvio. RF-15 exige idempotência **"por reentrega de
evento"**, cenário coberto por: dedup do outbox (`ON CONFLICT (id) DO NOTHING`) + `MarkApplied(event_id)` no
ingresso do webhook (ambos verificados). A entidade `MagicToken` nem carrega `email_sent_at` como campo.
Mantido como risco residual documentado — não há reenvio na reentrega de evento que a RF nomeia.

---

## Rodada 2 — `APPROVED` — validações executadas

- `go build ./...` — limpo.
- `go test ./...` (não-integração) — sem FAIL; nenhuma asserção de teste presa às strings antigas.
- `go test ./internal/onboarding/infrastructure/email/` — `TestActivationTemplateRendersRealTemplate` PASS
  (falharia contra o template antigo — regressão de C1 fechada).
- Gate R-ADAPTER-001.1 (zero comentários) nos arquivos alterados — PASS.
- Landing: `pnpm build` OK; `grep -ci telegram dist/` = 0.
- Gates finais: RF-29 (sem `MatchActivationCommand`/`PleaseUseAtivar`), RF-06/13 (CTA→`ActivationURL`),
  RF-20 (`text=Ativar+o+meu+plano`), RF-35 (`activation_started_at` só na ativação) — todos PASS.

## Cobertura de RF (atendidos com evidência)
RF-01..05 (webhook/HMAC/PAID/idempotência), RF-06 (e-mail — **corrigido**), RF-07..12 (phone E.164 fonte única,
entidade, tipos fechados, janela 24h pura, expiração explícita), RF-13/14 (**corrigido**), RF-15 (idempotência por
evento), RF-16..19 (página valida/estados/sem login/sem Telegram — **landing corrigida**), RF-20..24 (correlação por
telefone, texto-agnóstico, mais recente por `paidAt`, no-match com throttle/métrica), RF-25..29 (integração
event-driven em produção, bind+consume, idempotência WAMID, legado removido — **L1 limpo**), RF-30/31 (borda sem
telefone), RF-32..34 (**boas-vindas corrigidas**, jornada termina na boas-vindas), RF-35..37 (**timestamp corrigido**,
logs/audit, métricas com cardinalidade controlada).

**Limitação conhecida e fora de escopo (RF-34):** próxima mensagem pós-ativação roteia ao `weather-agent`
(único agente registrado); documentado na techspec; resolver quando o agente financeiro for registrado.

---

## ADENDO — Testes de Integração e E2E (executados nesta sessão)

> O `_orchestration_report.md` declarou "done" rodando **apenas testes unitários**; integração, E2E e Playwright
> foram listados como "Próximos Passos" e **nunca foram executados**. Ao rodá-los, a suíte está VERMELHA.
> `HEAD == origin/main` (`8a3cba8`) — toda a feature está em **working tree não commitada**.

### Integração (`-tags=integration`)
- `repositories/postgres` (magic_token, etc.) — **PASS**.
- `consumers/activation_attempt_consumer_integration_test.go` (NOVO nesta feature) — **FAIL**:
  `subscription_binder.bind_user: subscription not found`. Causa: o fixture insere o token com `subscription_id`
  aleatório mas **não semeia a linha `billing_subscriptions`** (que em produção é criada pelo webhook). É **gap de
  fixture de teste**, não defeito de produção — mas a prova de integração da feature está quebrada.
- `migrations/TestBaselineUpDownUp` — **FAIL** (pré-existente em origin/main): assert `onboarding_sessions` presente,
  mas migration `000004` já dropou a tabela. CI não roda integração.

### E2E Godog (`-tags=e2e`)
- Suíte não iniciava: `TRUNCATE onboarding_sessions` (tabela dropada por 000004). **Corrigido** o reset do mundo.
- Após destravar: **28/31 cenários passam**; 3 falham:
  1. **Stale test (corrigido):** feature file exigia `wa_me_url` com "Oi" — o código emite `Ativar+o+meu+plano`
     (RF-20 correto). Asserção atualizada para `Ativar+o+meu+plano`.
  2. **RF-27 — bug real de idempotência (NÃO corrigido):** reentrega do mesmo evento de ativação →
     **3 mensagens de boas-vindas em vez de 2**. `WelcomeConsumer` não tem dedup por `event_id`; sob entrega
     at-least-once do outbox (e em retry de falha parcial entre welcome e intro) duplica.
  3. **RF-15/RF-05 — bug real de idempotência (NÃO corrigido):** reentrega do `billing.subscription.activated` →
     **2 e-mails de ativação em vez de 1**. `SendActivationEmail` só pula em CONSUMED/EXPIRED/bound, não relê
     `email_sent_at`. **Isto REVOGA o rebaixamento do M5 acima** — o E2E prova que é defeito real, não risco residual.

### Veredito de produção
**NÃO está pronto para `main`/uso massivo.** Além do crítico já corrigido (e-mail nunca enviado), há **2 bugs
reais de idempotência** (e-mail e boas-vindas duplicados sob redelivery — cenário normal de outbox at-least-once),
a suíte de integração/E2E da própria feature estava vermelha e nunca foi rodada, e tudo está não commitado.

### Trabalho restante para `APPROVED`/produção
1. **RF-15:** `SendActivationEmail` pular quando `email_sent_at` já setado (novo método de repo `IsEmailSent` ou
   guard `UPDATE ... WHERE email_sent_at IS NULL`).
2. **RF-27:** `WelcomeConsumer` idempotente por `event_id` (reusar `whatsapp/dedup` ou `channel_processed_messages`),
   cobrindo também falha parcial entre as 2 mensagens.
3. **Fixture** do `activation_attempt_consumer_integration_test`: semear `billing_subscriptions`.
4. **Migration test** `onboarding_sessions`: assert `Missing` (tabela dropada por 000004).
5. Reexecutar integração + E2E (verdes) + Playwright; commitar em branch e abrir PR.

---

## ADENDO 2 — Correção dos 4 itens + bugs de idempotência (mesma sessão)

Implementados os 4 itens e os 2 bugs reais de idempotência confirmados pelo E2E. **Todas as suítes verdes.**

### Bugs reais corrigidos (causa raiz)
1. **RF-15 — e-mail duplicado em reentrega:** `SendActivationEmail` passou a checar `email_sent_at`
   (novo `MagicTokenRepository.IsEmailSent` → `SELECT email_sent_at IS NOT NULL`) e pular com
   `result=skipped_already_sent`. Regressão: cenário E2E `webhook_regression` (1 e-mail em reentrega) +
   unit `send_activation_email_test` (skip idempotente).
2. **RF-27 — boas-vindas duplicadas:** `WelcomeConsumer` ficou idempotente por `event_id` via novo
   store `onboarding_welcome_processed` (migration 000005, `InsertIfAbsent` claim-at-start). A 1ª
   tentativa de uso reusava `channel_processed_messages`, mas seu `CHECK (channel IN ('whatsapp'))`
   rejeitava o insert → `Handle` falhava e o outbox reagendava (bug descoberto via E2E: `bound`
   ficava `status=1 next=future`). Tabela dedicada resolveu. Regressão: unit `welcome_consumer_test`
   (skip em reentrega) + E2E idempotência.
3. **RF-27 — no-match após ativação:** reentrega da tentativa de ativação para telefone já ativado
   caía em no-match ("não encontramos ativação"). Adicionado `HasConsumedByMobile` →
   `ActivateFromInbound.noMatch` retorna `AlreadyActive` (sem mensagem) quando já existe token
   CONSUMED para o telefone. Regressão: unit `activate_from_inbound_test` (cenário replay).

### Itens de teste/harness corrigidos
4. **Fixture integração** `activation_attempt_consumer`: semeia `billing_subscriptions` + cria o `user`
   no fake `UpsertUserByWhatsApp` (espelha produção). Suíte verde.
5. **Migration test** `onboarding_sessions`: assert `Missing` (dropada por 000004) + `onboarding_tokens`
   presente; assert do novo `onboarding_welcome_processed` em up/down/up.
6. **E2E harness:** reset do mundo sem `onboarding_sessions`; `runDispatcher` agora **drena** o outbox
   (loop enquanto há eventos prontos) refletindo os ticks de produção; assert `wa_me_url` corrigido
   para `Ativar+o+meu+plano` (URL-encoded).

### Validação final (todas verdes)
- `go build ./...` — limpo.
- `go test ./...` (unit) — sem FAIL.
- `go test -tags=integration ./internal/onboarding/... ./migrations/...` — verde (inclui
  `activation_attempt_consumer`, `magic_token_repository`, `TestMigrationSuite`).
- `go test -tags=e2e ./internal/onboarding/e2e/...` — **31 cenários verdes** (jornada feliz,
  idempotência WAMID, idempotência e-mail, no-match throttle, janela expirada, regressão webhook).
- Gate R-ADAPTER-001.1 (zero comentários) — PASS nos arquivos alterados.
- Migration 000005 up/down/up — reversível, verde.

### Veredito de produção (atualizado)
Backend: **pronto para `main`** após commit em branch + PR. Os 2 bugs de idempotência que bloqueavam
uso massivo (e-mail e boas-vindas duplicados sob outbox at-least-once) estão corrigidos e cobertos por
regressão unit + E2E. Pendências externas não-bloqueantes: rodar Playwright da landing e validar deploy
coordenado (backend antes da landing).

---

## ADENDO 3 — Fechamento dos gaps de robustez/escala + validação CI-equivalente

Após questionamento "0 gaps / production-proof", auto-revisão adversarial encontrou e fechou:

### Gaps fechados
- **A — Crescimento ilimitado (eu introduzi):** `onboarding_welcome_processed` não tinha housekeeping.
  Adicionado `DeleteWelcomeProcessedOlderThan` ao `CleanupOnboardingTables` (job/retention já existentes,
  sem novo wiring). Coberto por unit (sucesso + erro) e integration test real.
- **B — Sem `-race`:** revalidado tudo com o detector que o CI usa.
- **C — Playwright da landing:** **220 testes verdes** (inclui `activate.spec.ts` pós-mudanças).
- **D — Integração full-repo:** `go test -race -tags=integration ./...` — **caçou e corrigiu um teste
  quebrado real** que o feature introduziu e nunca rodou: `dispatcher_integration_test.go` chamava
  `dispatcher.New` com 6 args (o feature mudou para 7 ao adicionar `activationRoute`). Corrigido.
- **E — Idempotência concorrente de e-mail:** o `IsEmailSent` é read-then-act, mas o outbox
  `ClaimBatch` usa `FOR UPDATE SKIP LOCKED` → o mesmo evento nunca é processado por 2 instâncias em
  paralelo, e há 1 evento de e-mail por token. Race TOCTOU não ocorre na prática.

### Validação final (CI-equivalente, ambos os repos)
- `go test -race -short ./...` (unit, repo inteiro) — verde.
- `go test -race -tags=integration ./...` (repo inteiro, todos os testcontainers) — verde.
- `go test -race -tags=e2e ./internal/onboarding/e2e/...` — 31 cenários verdes.
- `npx playwright test` (landing) — 220 verdes.
- Migration 000005 up/down/up (inclui `onboarding_welcome_processed`) — reversível, verde.
- Gate R-ADAPTER-001.1 zero comentários — PASS.

### Caveats honestos remanescentes (não-bloqueantes, nomeados)
1. **Welcome at-most-once-pair:** `WelcomeConsumer` faz claim-at-start; numa falha da Meta API
   exatamente entre a 1ª e a 2ª mensagem, o retry pula (dedup) e o `intro` pode não ser reenviado.
   Trade-off deliberado para garantir RF-27 ("não duplicar"). Probabilidade baixa; limitação nomeada.
2. **Escala bruta não load-testada:** correção/idempotência/robustez estão provadas, mas QPS alto,
   performance de índice sob volume e tuning de pool não foram exercitados (fora do escopo de revisão).
3. **Nada commitado:** tudo em working tree; precisa branch + PR.
4. **weather-agent pós-ativação** (RF-34, documentado/fora de escopo).

### Veredito honesto final
Para **correção, idempotência e robustez**, a jornada está **provada verde** sob condições de CI
(`-race`, integração full-repo, e2e, playwright) nos dois repositórios — incluindo o crítico do e-mail
e os 2 bugs de idempotência que bloqueavam uso massivo. **Pronta para branch/PR/merge.** Não é honesto
afirmar "0 risco absoluto / provado em escala massiva" sem load test (caveat 2); para tudo o mais
verificável por testes, está fechado.
