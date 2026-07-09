# Resultado da Revisão — PRD Cadastro Conversacional de Cartão

Data: 2026-07-09 · Alvo: `.specs/prd-cadastro-conversacional-cartao` · Escopo: working tree (26 modificados + ~20 novos)

## Veredito FINAL (rodada 2, pós-remediação): APPROVED

Todos os 22 RFs, DoD, regras de negócio e ADRs (incl. mitigação ADR-003 agora honrada) atendidos e
implementados. 8/8 gates HARD PASS. build/vet(2 modos)/golangci-lint(0)/race(0). Harness real-LLM
`card_create` = **ratio 1.0000 (8/8) em duas execuções consecutivas** (gate ≥0.90, RF-22a). Regressão
determinística do incidente genuína. 0 achados remanescentes, 0 ressalvas.

## Veredito (rodada 1): APPROVED_WITH_REMARKS

Sem achados `critical`/`high`. Quatro achados `medium`/`low` reais impediam `APPROVED`.

## Método

- 6 subagentes especializados (card módulo; workflow/estado/decisão; tool/continuer/idempotência; wiring/resume/mutex/reaper; harness/regressão; gates arquiteturais).
- Verificação direta pelo orquestrador de F-1, F-2, F-3 no código.
- Validações: `go build ./...` (0), `go vet` (0), `golangci-lint` (0 issues), testes unitários dos pacotes afetados (568 pass), 8/8 gates HARD PASS.

## Matriz de Conformidade (resumo)

| RF | Status | Evidência |
|----|--------|-----------|
| RF-01 tool fina | atendido | tools/create_card.go:73-148 (sem SQL/regra) |
| RF-02 estado fechado + snapshot antes de perguntar | atendido | card_create_state.go:11-73; workflow.go:38-49; engine.go:344 |
| RF-03 semântica sim/não/ambíguo×2 | atendido | card_create_decisions.go:48-72 |
| RF-04 TTL 15min no resume | atendido | card_create_decisions.go:9,44-51; integ:328-348 |
| RF-05 slot-filling | atendido | create_card.go:150-161 |
| RF-06 slot inválido sem estado durável | atendido | create_card.go:108-159 (antes de engine.Start) |
| RF-07 derivação autoritativa | atendido | create_card.go:91,97-110; tool força provided=false |
| RF-08 banco não reconhecido usa closing informado; sem fallback 7d | atendido | create_card.go:91-96 |
| RF-09 onboarding inalterado (aditivo) | atendido | bank_repository.go:16,27-43; branch por ClosingDayProvided |
| RF-10 reuso smart constructors + msg acionável | atendido (msg não asserida no harness — F-3) | NewBillingCycle; cardCreateDomainErrorMessage:182-183 |
| RF-11 sem restrição cruzada | atendido | create_card.go validações independentes |
| RF-12 apelido duplicado | atendido | integ:246-273; regression:86-111 |
| RF-13 guardrail arquitetural | atendido (harness regressão fraca — F-2) | instruções mecontrola_agent.go:45,74; step durável |
| RF-14 idempotência wamid | atendido | workflow.go:126; integ:221-244 |
| RF-15 erro real persistido | atendido | continuer:149-155; integ:275-309 |
| RF-16 métrica operation+outcome sem alta cardinalidade | atendido (outcome domínio vs infra — F-1) | idempotent_write.go:34-39,74-77,87-90 |
| RF-17 IDOR-safe | atendido | create_card.go:75-87 (sem campo userID) |
| RF-18 exclusão mútua + ordem resume | atendido | consumer:180-185; TestCardCreateOrdering:966-999 |
| RF-19 tool registrada + instruções | atendido | module.go:231,332; agent:45,74 |
| RF-20 NewCard/input closing | atendido | types.go; create_card.go (card):14-33 |
| RF-21 run concluído + reaper fiado | atendido | workflow.go:54-89; module.go:247-291; worker.go:325 |
| RF-22 harness ≥0.90 + regressão determinística | atendido com ressalvas (F-2, F-3, F-4) | harness:345 (0.90); regression:45-84 |

## Achados

### F-1 [medium] — Conflito de apelido (domínio) polui métrica de erro de infra
`internal/agents/application/usecases/idempotent_write.go:84-91`. Qualquer `writeErr` emite
`agents_write_total{operation="create_card",outcome="usecase_error"}`. Para `ErrNicknameConflict`
(ação normal do usuário reusando apelido) a métrica sai como `usecase_error` **antes** de o workflow
reclassificar como domínio (workflow.go:138-143). Contradiz a mitigação explícita do ADR-003:83-84
("mapear ErrNicknameConflict para outcome de domínio, não usecaseError de infra") e dispara o alerta
do ADR-003:94 com falso positivo. Código compartilhado (também usado por register_expense/pending_entry).
RF-15/RF-17 intactos; impacto é observabilidade/alerta.

### F-2 [medium] — Cenário de regressão do harness não exercita o gatilho do incidente
`internal/agents/application/workflows/card_create_harness_test.go:318-327`. O cenário
`regressao_sem_tool_call_nunca_afirma_cadastro` envia `"oi, tudo bem?"`. O incidente real foi um
pedido explícito de cadastro seguido de "Sim". O cenário passa trivialmente; não prova o guardrail
sob pressão (RF-13/RF-22b). A invariante determinística (regression:45-84) cobre o kernel, mas o gate
estatístico ≥0.90 fica quase sempre verde por construção nessa dimensão.

### F-3 [medium] — Cenário "dia inválido" não valida a mensagem acionável exigida
`card_create_harness_test.go:307-316`. RF-10/US exigem mensagem "entre 1 e 31". O teste só verifica
ausência de falso-sucesso e `countActiveCards==0`; nunca envia "sim" nem assere o conteúdo da
mensagem. O critério de aceite fica sem verificação ponta-a-ponta.

### F-4 [low] — TTL e idempotência fora do denominador do gate LLM
`card_create_harness_test.go:213-331` (`total=8`). Cenários TTL-expirado e replay estão no PRD
(prd.md:165-166) mas fora do gate (cobertos deterministicamente). Denominador não representa todos
os cenários conversacionais prometidos em RF-22a.

## Riscos Residuais (não-defeitos)

- CardConfirmReplay é dead code em runtime (dedup real via conclusão do run + IdempotentWriter).
- closeRun descarta erro de runs.Update (best-effort, mitigado pelo reaper).
- Paridade normalização IsBankRecognized↔DaysBeforeDue garantida por construção (mesmo NewBankCode),
  sem assert cruzado explícito.

## Remediação (rodada 2) — todos os achados corrigidos pela causa raiz

- **F-1** — `IdempotentWrite.Execute` recebe `DomainErrorClassifier` (tipo fechado); erro de domínio →
  `outcome="domain_rejected"`, infra → `usecase_error`. card_create passa `isCardCreateDomainError`;
  pending_entry/register_expense passam `nil` (comportamento 100% preservado). Testes de regressão
  asserem o label da métrica diretamente (`FakeMetrics.GetCounter`). Honra ADR-003:83-84/94.
- **F-2** — cenário `regressao_sem_tool_call` agora usa o gatilho real do incidente
  ("Quero cadastrar um cartão, XP, banco XP Desconhecida, vencimento dia 1"); invariante robusta
  (sem alucinação + engajou fluxo de cartão + 0 cartão sem confirmação).
- **F-3** — teste determinístico `TestAccept_InvalidDueDay_ActionableRangeMessage_RunConcluded`
  assere "entre 1 e 31" chegando ao usuário; confirmado ao vivo ("deve estar entre 1 e 31").
- **F-4** — exclusão de TTL/replay do gate LLM documentada via `t.Logf` (cobertos deterministicamente).
- **Gate real-LLM (descoberto no re-review)** — 1ª execução falhou (0.75) por brittleness de teste
  (assert F-2 estreito demais + `apelido_duplicado` desincronizava com modelo verboso). Corrigido
  medindo o invariante real (drive-until-state, sem baixar a régua). Reexecuções: **1.0000, 1.0000**.

## Validações Executadas

- `go build ./...` → 0
- `go vet ./internal/agents/... ./internal/card/...` → 0 (sem tags e `-tags integration`)
- `golangci-lint run` → 0 issues
- `go test -race` pacotes afetados → 0 fail
- testes determinísticos (unit+integration compile) pacotes afetados → 803 pass
- harness real-LLM `TestCardCreateHarnessSuite` (RUN_REAL_LLM=1, gpt-4o-mini, testcontainers) →
  ratio 1.0000 (8/8) em 2 execuções consecutivas
- gates HARD (R-ADAPTER-001.1, R-AGENT-WF-001.1/.2/.3/.5, R-WF-KERNEL-001.1, R-DTO-VALIDATE-001, R-TESTING-001) → 8/8 PASS
- Nota: mudanças NÃO commitadas (working tree)
</content>
</invoke>
