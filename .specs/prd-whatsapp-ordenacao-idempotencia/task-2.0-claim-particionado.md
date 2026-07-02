# Tarefa 2.0: Claim particionado no ClaimBatch + captura de SQLSTATE 23505

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reescrever o `ClaimBatch` do adapter Postgres do outbox para reivindicar no máximo 1 evento em voo por
`aggregate_user_id`, ordenado por `occurred_at` (timestamp da Meta), preservando FIFO por usuário sem
segurar conexão durante o LLM. Capturar a colisão de índice único (`SQLSTATE 23505`) e adiar o lote
(RF-01, RF-02, RF-03). Sem mudar a assinatura pública do repositório.

<requirements>
- RF-01: serializar por `aggregate_user_id` — no máximo 1 evento `status=2` em voo por usuário, mesmo com 2+ workers.
- RF-02: nenhuma conexão/transação aberta durante o LLM; PROIBIDO `pg_advisory_lock` de sessão. O claim opera em transação curta (claim + mark).
- RF-03: FIFO por usuário — reivindicar apenas eventos de usuários sem evento em voo e sem pendente anterior (`NOT EXISTS` por `occurred_at`).
- Ordenação `ORDER BY occurred_at`, `FOR UPDATE SKIP LOCKED`, marcando `status=2` na mesma transação; `RETURNING` inclui `aggregate_user_id`, `occurred_at`, `metadata`.
- Eventos sistêmicos (`aggregate_user_id IS NULL`) não são serializados.
- D-14: capturar `SQLSTATE 23505` (violação de `outbox_events_user_inflight_uidx`) e **descartar o lote** (adiar para o próximo tick de 500ms); NÃO tratar como erro fatal, os eventos seguem pendentes.
- SQL exclusivamente no adapter Postgres (R-ADAPTER-001, R-WF-KERNEL-001.2); zero comentários; kernel/outbox genérico (chave opaca `aggregate_user_id`, sem domínio).
</requirements>

## Subtarefas

- [ ] 2.1 Reescrever a query do `ClaimBatch` (CTE `claimable` + `UPDATE ... FROM claimable ... RETURNING`) conforme techspec §Modelos de Dados.
- [ ] 2.2 Detectar `SQLSTATE 23505` (via `*pgconn.PgError` / `errors.As`) no `ClaimBatch` e retornar lote vazio sem erro (adiar), com métrica/observabilidade de "adiado por colisão".
- [ ] 2.3 Garantir que o `RETURNING` traga `aggregate_user_id`/`occurred_at`/`metadata` para os consumidores downstream.
- [ ] 2.4 Testes unitários (testify/suite) de repositório com fixtures cobrindo os cenários de concorrência.

## Detalhes de Implementação

Ver ADR-001 §Decisão/§Plano de Implementação e techspec §Modelos de Dados (bloco `WITH claimable AS ...`)
e a Nota de robustez (RF-01/D-14: `UPDATE ... FROM claimable` é atômico por statement; a violação
`23505` aborta o lote inteiro → capturar e adiar). Poison head-of-line (RF-22) é consequência natural do
`NOT EXISTS` por `occurred_at` — o dimensionamento de dead-letter fica na tarefa 7.0.

## Critérios de Sucesso

- 3 eventos do mesmo usuário liberam 1 por vez; usuários distintos processam em paralelo.
- Evento sistêmico (`user_id NULL`) nunca bloqueia nem é bloqueado.
- Ordenação por `occurred_at` respeitada.
- Colisão worker-1/2 vira lote vazio (adiado), nunca erro propagado.
- Nenhum lock de sessão; transação curta libera a conexão antes do LLM.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Unitários: suite sobre o repositório com fixtures (3 eventos do mesmo usuário → 1 por vez; usuários
distintos em paralelo; `user_id NULL` não bloqueia; ordenação por `occurred_at`; simulação de colisão
→ adiar). A verificação de concorrência real de ponta a ponta (2 workers) é a CA-01 na tarefa 8.0.

## Rollback

Reverter o `ClaimBatch` ao `ORDER BY next_attempt_at ... FOR UPDATE SKIP LOCKED` anterior e dropar os
índices (migration down da tarefa 1.0) — comportamento antigo restaurado (ADR-001 §Riscos/Rollback).

## Done-when

- Suite unitária verde nos 5 cenários.
- `ClaimBatch` não segura conexão durante o processamento (transação curta comprovada no código).
- Colisão `23505` capturada e adiada (teste específico).
- Validação proporcional (SQL concorrente no adapter): `go build ./...`, `go vet ./internal/platform/outbox/...`, `go test -race -count=1 ./internal/platform/outbox/...`.

## Arquivos Relevantes
- `internal/platform/outbox/storage_postgres.go` (`ClaimBatch`)
- `internal/platform/outbox/outbox.go`, `internal/platform/outbox/dispatcher.go`, `internal/platform/outbox/status.go`
- `configs/config.go` (OutboxConfig)
