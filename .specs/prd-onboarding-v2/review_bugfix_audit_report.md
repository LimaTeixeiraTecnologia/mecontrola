# Auditoria Review + Bugfix — Onboarding V2 (DoD / Critérios de Aceite)

- Data: 2026-06-24
- Escopo: confronto de TODOS os RF-01..RF-36 (+RF-13a), EB-01..EB-16, DR-01..DR-11 contra o diff real
- Método: 6 revisores especializados em paralelo (por fatia de RFs) → bugfix com subagents → re-review adversarial do delta → hardening → validação consolidada
- Estado final: **APROVADO PARA MAIN** — 136/136 pacotes PASS, 0 FAIL; gates de fronteira CLEAN

## Veredito por fatia (Ciclo 1)

| Fatia | Veredito inicial | Resultado final |
|-------|------------------|-----------------|
| Domínio + persistência (RF-19/20/22/31/35, EB-10/15, DR-08, VO perfis) | APPROVED | mantido |
| Lifecycle usecases (RF-23/24/25/29/30/34/35, EB-07/08, DR-09) | APPROVED_WITH_REMARKS | gofmt corrigido |
| Budgets + card (RF-12/13/13a, EB-13/14/16, DR-05/06/11) | APPROVED_WITH_REMARKS | testes + DTO corrigidos |
| Agent turn/scripts (RF-09..18, RF-33/36, EB-05/13, RF-08/32) | REJECTED (H1,H2) | corrigido + re-verificado |
| Consumers greeting/WM (RF-01..08/26/29/32/34, EB-01/02/03/09/11, DR-02/10) | REJECTED (F1,F3 / F2,F4) | F1 falso-positivo; F2/F3/F4 corrigidos |
| Gates arquiteturais (ADR-006, R-AGENT-WF-001, R-ADAPTER-001, state-as-type) | APPROVED | mantido |

## Falso positivo eliminado (verificação adversarial)

- **F1 / RF-07** (alegado: app sobe sem modelo de LLM de onboarding). **FALSO POSITIVO.** `configs/config.go:1142-1143` já falha o boot com erro explícito `"AGENT_ONBOARDING_LLM_MODEL é obrigatório — onboarding sem LLM é proibido"` quando onboarding está habilitado. O revisor olhou só `module.go` e perdeu a camada de validação de config. RF-07 atendido. **Não corrigido (corretamente).**

## Correções aplicadas (root-cause + teste de regressão)

| ID | Sev | RF/EB | Correção | Arquivo |
|----|-----|-------|----------|---------|
| H2 | high | RF-16 | `cardsPhase` recarrega snapshot (`reader.Load`) antes do resumo Etapa 4/4; cartão do mesmo turno passa a aparecer | run_onboarding_turn.go |
| H1 | high | RF-36/RF-11 | prompt de off-topic instrui re-exibir `🔵 Etapa X/4 — <Nome>` (literal techspec) | onboarding_scripts.go |
| F3 | high | RF-32/EB-03 | `RouteResult.Delivered`; consumer retorna erro→retry e NÃO marca `welcome_sent_at` em falha de entrega | intent_router.go, onboarding_bound_consumer.go |
| F2 | medium | RF-29 | idempotência autoritativa: erro só quando AMBAS as escritas falham (força retry) | onboarding_bound_consumer.go |
| F4 | medium | DR-10 | métrica `outbox_dead_letter_total{event_type}` no branch de esgotamento de retry; `NewObservableDispatcherJob` + wiring | platform/outbox/dispatcher.go, cmd/worker/worker.go |
| RF29-WIN | medium | RF-29/EB-02 | check read-only de `welcome_sent_at` pré-route (snapshot.WelcomeSent) fecha janela de falha parcial assimétrica | get_onboarding_context.go, run_onboarding_turn.go, onboarding_state_reader.go, onboarding_bound_consumer.go |
| C1 | low | EB-14 | DTO `create_card` rejeita `closing_day <1 || >31` (simétrico a due_day) | card/.../input/create_card.go |
| C2 | medium | DR-11/RF-12 | teste da derivação de `due_day` (nil→closing+7, wrap -30) | create_card_test.go |
| C3 | low | DR-05 | teste do branch opcional `due_day==0→nil` no consumer | onboarding_card_consumer_test.go |
| FMT | medium | — | gofmt em 9 arquivos (gate de lint) | vários |

## Validação consolidada

- `go build ./...` → OK
- `go vet` (módulos afetados) → limpo
- `go test ./...` → **136 pacotes PASS, 0 FAIL**
- `gofmt -l` (todos os arquivos tocados) → vazio (limpo)
- Gates: `buildAutoSplits` CLEAN · `OnboardingLLMEnabled` CLEAN · SQL em adapters CLEAN · zero-comentários CLEAN · RF-07 boot guard PRESENT · dead-letter metric PRESENT · `daily_ledger` cases=0

## Riscos residuais (não bloqueantes, documentados)

- `dedupTotal{result=sent}` pode subcontar quando `RegisterGreeting` sucede mas `MarkWelcomeSent` falha (observabilidade, não comportamento). low.
- `TestRunDeadLetterMetric` assegura que o branch dead-letter é alcançado, mas não asserta o valor do contador (o fake garante non-nil). low.
- RF-14 (recalcular só diferenças): o ajuste reenvia as 5 categorias e revalida soma==renda (design full-replace validado); progresso preservado no draft. Conforme techspec/DR-06.
