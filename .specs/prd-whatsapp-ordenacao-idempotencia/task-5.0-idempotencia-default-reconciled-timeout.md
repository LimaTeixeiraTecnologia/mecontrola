# Tarefa 5.0: Idempotência default + mapa reconciled + timeout de coerência + remoção do advisory lock

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ligar a idempotência por padrão (remover o gate `WriteAdvisoryLock` e deletar `advisory_key_locker.go`),
garantir que o conflito da chave natural de domínio propague como `ToolOutcomeReconciled`/replay (nunca
`usecaseError` nem sucesso falso), e impor timeout de contexto de LLM/tool ≪ `STUCK_AFTER` como
hardening de coerência (RF-04, RF-05, RF-20, RF-21; ADR-002 emenda v3).

<requirements>
- RF-04: idempotência por `(wamid, item_seq, operation)` ativa por padrão (sem depender de flag de ambiente), com `agents_write_ledger` como fonte de verdade de replay do agente.
- RF-05: redelivery com o mesmo `message_id` não produz segunda escrita nem resposta duplicada/contraditória.
- RF-20 [PRESERVAR, não recriar]: a idempotência de escrita de domínio JÁ EXISTE em produção (`transactions_origin_uk`, `transactions_card_purchases_origin_uk`, `origin` cabeado ponta-a-ponta, usecase retorna `Reconciled`). NÃO criar migration de chave natural nova. A tarefa é: (a) mapear o conflito de origem para `agent.ToolOutcomeReconciled`/replay em `idempotent_write.go` (nunca `usecaseError`); (b) gate de revisão exigindo `origin` + UNIQUE natural equivalente em qualquer NOVA tool de escrita.
- RF-21: timeout de contexto na chamada de LLM/tool estritamente menor que `STUCK_AFTER` (ex.: 90s < 5m) para o worker original concluir/liberar antes de o reaper resetar o evento (`status=2→1`), evitando re-pick concorrente que gere 2ª resposta fora de ordem. Reserva ledger-first é opcional (redundante com a chave natural).
- Remover o gate `AGENT_WRITE_ADVISORY_LOCK` em `cmd/worker/worker.go` e **deletar** `internal/agents/infrastructure/persistence/advisory_key_locker.go` (caminho `pg_advisory_lock` de sessão — redundante e inseguro sob pgbouncer transaction-pool). Remover a dependência do `KeyLocker` em `idempotent_write.go`.
- Serialização = claim particionado (tarefa 2.0) + UNIQUE do `agents_write_ledger`; nenhum lock auxiliar de sessão.
- Zero comentários; tipos fechados; `errors.Join`/`%w`; sem abstrair tempo.
</requirements>

## Subtarefas

- [ ] 5.1 `idempotent_write.go`: remover dependência do `KeyLocker`; garantir que o conflito da chave natural de domínio propague como `ToolOutcomeReconciled`/replay (nunca `usecaseError`).
- [ ] 5.2 `cmd/worker/worker.go`: remover o gate `WriteAdvisoryLock` (idempotência default) e impor timeout de contexto de LLM/tool ≪ `STUCK_AFTER`.
- [ ] 5.3 Deletar `advisory_key_locker.go` e ajustar wiring que o referenciava.
- [ ] 5.4 Gate de revisão (documentado): toda nova write tool DEVE carregar `origin` + UNIQUE natural equivalente no alvo.
- [ ] 5.5 Testes unitários (testify/suite): replay/reconciled/usecaseError; ausência de flag; timeout dispara antes do reaper.

## Detalhes de Implementação

Ver ADR-002 §Decisão (item 5) e §Emenda v3 (itens 6–8), techspec §"Idempotência de domínio (RF-20) e
hardening de coerência (RF-21)" e §Arquivos Relevantes. Depende da tarefa 4.0 (write tools já
propagando `ToolOutcome`). A verificação sob corrida real (reset do reaper durante `write()` lento) é a
CA-09 na tarefa 8.0.

## Critérios de Sucesso

- Idempotência ativa sem flag; redelivery → 1 linha em `agents_write_ledger`, 0 duplicatas.
- Conflito de `origin` → `reconciled`/replay; nunca `usecaseError` nem sucesso falso.
- `advisory_key_locker.go` removido; nenhum `pg_advisory_lock` de sessão no código.
- Timeout de LLM/tool < `STUCK_AFTER` configurado e testado.
- Gate de revisão de `origin`+UNIQUE documentado para novas tools.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — altera `idempotent_write` (usecase agent-owned), as write tools e o wiring do worker do agente; a skill cobre idempotência e o ciclo de escrita do agente sobre `internal/agents`.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Unitários testify/suite para `idempotent_write` (replay/reconciled/usecaseError) e a config de timeout.
CA-02 (redelivery) e CA-09 (corrida de domínio) são integração na tarefa 8.0.

## Rollback

Reverter a remoção do gate e restaurar `advisory_key_locker.go` (git revert) volta ao comportamento
anterior; a chave natural do domínio permanece intacta (não foi tocada).

## Done-when

- Suites unitárias verdes; sem flag `AGENT_WRITE_ADVISORY_LOCK` no código.
- Gate executável de remoção destrutiva (deve retornar vazio):
  ```bash
  grep -rn "pg_advisory_lock\|advisory_key_locker\|AGENT_WRITE_ADVISORY_LOCK" --include="*.go" internal/ cmd/ \
    && echo "FAIL: advisory lock não removido completamente" && exit 1 || true
  ```
- Timeout < `STUCK_AFTER` verificável na config.
- Validação proporcional (mudança em usecase + wiring): `go build ./...`, `go vet ./internal/agents/... ./cmd/worker/...`, `go test -race -count=1 ./internal/agents/application/usecases/...`.

## Arquivos Relevantes
- `internal/agents/application/usecases/idempotent_write.go`
- `internal/agents/infrastructure/persistence/advisory_key_locker.go` (REMOVIDO)
- `cmd/worker/worker.go` (gate `WriteAdvisoryLock`, timeout de contexto)
- `internal/agents/application/tools/{register_expense,register_income,register_card_purchase}.go` (gate origin+UNIQUE)
