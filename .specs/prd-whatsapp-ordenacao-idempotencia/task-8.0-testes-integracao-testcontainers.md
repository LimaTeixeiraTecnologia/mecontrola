# Tarefa 8.0: Testes de integração (testcontainers) — concorrência, idempotência, onboarding, poison

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Provar, com Postgres real via `testcontainers-go` (build tag `//go:build integration`), as garantias de
concorrência, idempotência ponta-a-ponta, confirmação honesta, onboarding resume, idempotência de
domínio sob corrida, ingestão multi-mensagem e dead-letter de poison (CA-01, CA-02, CA-03, CA-04,
CA-07, CA-09, CA-10).

<requirements>
- CA-01: N mensagens rápidas do mesmo usuário com 2 workers ativos → zero execução concorrente por usuário (via timeline de `platform_runs`) e ordem FIFO preservada; nenhum passo segura conexão durante o LLM.
- CA-02: reprocessar o mesmo evento inbound (redelivery) → 1 linha em `agents_write_ledger`, 0 lançamentos/respostas duplicados.
- CA-03: forçar erro de persistência → nenhuma confirmação de sucesso e nenhum envio vazio (outbound vazio = 0); validação com LLM real (`RUN_REAL_LLM=1` + `OPENROUTER_*`).
- CA-04: duas `Start` concorrentes de onboarding → 1 run ativo, a 2ª retoma, 0 `onboarding_error`; turnos aparecem em `platform_messages`.
- CA-07: webhook com múltiplas mensagens processa todas na ordem do timestamp da Meta sob claim particionado.
- CA-09: dois workers no mesmo `(origin_wamid, origin_item_seq, origin_operation)`, simulando reset do reaper durante um `write()` lento → 1 mutação de domínio (a chave natural rejeita a 2ª, retorna `Reconciled`), outcome mapeia para `reconciled`/replay (nunca `usecaseError` nem sucesso falso), e o timeout de LLM dispara antes do `STUCK_AFTER`.
- CA-10: evento inbound poison vai a dead-letter (`status=4`) dentro do orçamento de `max_attempts` sem bloquear indefinidamente os seguintes do usuário; alerta de `status=4` observável; FIFO das mensagens não-poison preservado.
- Suites de integração podem usar `package <X>_test` e SQL direto em fixtures (exceção documentada de R-ADAPTER-001/R-TESTING-001).
</requirements>

## Subtarefas

- [ ] 8.1 CA-01: concorrência por usuário (2 workers, N eventos) — timeline via `platform_runs`, FIFO nas respostas.
- [ ] 8.2 CA-02: idempotência sob redelivery — 1 linha no ledger, 0 duplicatas.
- [ ] 8.3 CA-03: confirmação honesta sob erro de persistência (sem sucesso/sem vazio) + validação LLM real.
- [ ] 8.4 CA-04: Start idempotente-resume concorrente — 1 run, 2ª retoma, 0 error + turnos em `platform_messages`.
- [ ] 8.5 CA-07: webhook multi-mensagem processado na ordem do timestamp da Meta.
- [ ] 8.6 CA-09: idempotência de domínio sob corrida (reset do reaper durante `write()` lento) — 1 mutação, `reconciled`, timeout antes do `STUCK_AFTER`.
- [ ] 8.7 CA-10: poison → dead-letter (`status=4`) sem bloqueio de FIFO; alerta observável.

## Detalhes de Implementação

Ver techspec §Abordagem de Testes (§Testes de Integração, lista CA-01..CA-10) e os §Plano de
Implementação de ADR-001/002/003/005. Usar `testcontainers-go` com `//go:build integration`. Depende de
2.0 (claim), 3.0 (ingestão em lote), 4.0 (confirmação honesta — CA-03 testa a saída de 4.0
diretamente), 5.0 (idempotência/reconciled/timeout) e 6.0 (onboarding resume).

## Critérios de Sucesso

- CA-01, CA-02, CA-03, CA-04, CA-07, CA-09, CA-10 verdes com Postgres real.
- Zero execução concorrente por usuário; 0 duplicidade; 0 outbound vazio; 0 `onboarding_error` no cenário concorrente.
- Validação com LLM real registrada como evidência (CA-03).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — os testes exercitam o ciclo Thread→Run do agente, o loop de tool-calling e o onboarding sobre `internal/agents`/`internal/platform`; a skill é a base canônica desses fluxos e da validação real-LLM.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Esta tarefa É a suíte de integração (testcontainers). Executar com a build tag `integration` e, para
CA-03, com `RUN_REAL_LLM=1` + credenciais `OPENROUTER_*` do `.env`.

## Rollback

Testes são aditivos; nenhum rollback funcional. Falha de CA bloqueia o merge das tarefas de
implementação correspondentes (evidência de regressão).

## Done-when

- Todos os 7 cenários CA verdes no CI com testcontainers.
- Evidência de validação real-LLM anexada para CA-03.

## Arquivos Relevantes
- `internal/platform/outbox/*_integration_test.go` (CA-01, CA-10)
- `internal/agents/.../*_integration_test.go` (CA-02, CA-03, CA-04, CA-07, CA-09)
- `internal/platform/agent/*_integration_test.go` (confirmação honesta end-to-end)
- Helpers de testcontainers/fixtures Postgres
