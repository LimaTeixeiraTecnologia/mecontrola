# Tarefa 2.0: Busca de candidatos de edição (SearchEditCandidates)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar uma query combinada de candidatos de edição que localize lançamentos por valor exato OU termo de descrição, restrita ao mês vigente e ordenada por recência, limitada a um top-N pequeno. Expor via repositório, usecase e porta do agente, cobrindo os modos "era 25" (só valor) e "aquele mercado" (só termo).

<requirements>
- RF-15 (localizar o lançamento a corrigir por valor e/ou descrição).
- ADR-007 (busca de candidatos de edição por valor e/ou descrição).
- R-DTO-VALIDATE-001: input DTO com `Validate()`.
- R-ADAPTER-001: binding/adapter fino, sem regra de domínio nem SQL fora do repositório.
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar `SearchEditCandidates(ctx, userID, amountCents int64, term string, refMonth, limit)` à interface `internal/transactions/application/interfaces/transaction_repository.go` e implementar em `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go` (`WHERE user_id AND deleted_at IS NULL AND ref_month = $ AND (amount_cents = $amount OR description ILIKE '%'||$term||'%') ORDER BY created_at DESC LIMIT $`).
- [ ] 2.2 Criar usecase `search_edit_candidates.go` (em `internal/transactions/application/usecases/`) com DTO de input contendo `Validate()`.
- [ ] 2.3 Adicionar a porta `TransactionsLedger.SearchEditCandidates` + o value object de consulta `EditCandidateQuery` em `internal/agents/application/interfaces/transactions_ledger.go` e o binding em `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`.
- [ ] 2.4 Testes unitários (valor, termo, ambos vazios) e integração com testcontainers.

## Detalhes de Implementação

Ver techspec.md, seção `### Interfaces Chave` e o `### Mapeamento Requisito -> Decisão -> Teste` (RF-15), e ADR-007 desta pasta. Pontos-chave (não duplicar, referenciar):

- Query combina valor exato OU termo ILIKE, restrita a `ref_month`, `created_at DESC`, top-N pequeno (ex.: 5) (ADR-007).
- Índice de suporte `(user_id, ref_month, created_at)` é desejável para performance.
- `EditCandidateQuery` usa smart constructors; SQL vive exclusivamente no repositório postgres (R-ADAPTER-001).
- Havendo mais de um candidato, o fluxo de edição lista opções; havendo um, apresenta o registro (semântica consumida pelo fluxo `transaction-write`, fora desta tarefa).

## Critérios de Sucesso

- "era 25" (só valor) retorna candidatos que casam por `amount_cents`.
- "aquele mercado" (só termo) retorna candidatos que casam por `description ILIKE`.
- Resultado limitado a um top-N pequeno (ex.: 5), ordenado por recência dentro do mês vigente.
- Input DTO valida e rejeita consulta com valor e termo ambos vazios.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `postgresql-production-standards` — nova query SQL de candidatos de edição no repositório postgres.
- `domain-modeling-production` — value object de consulta (EditCandidateQuery) e smart constructors.

## Testes da Tarefa

- [ ] Testes unitários (valor, termo, ambos vazios)
- [ ] Testes de integração (testcontainers)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/transactions/application/interfaces/transaction_repository.go`
- `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go`
- `internal/transactions/application/usecases/search_edit_candidates.go`
- `internal/agents/application/interfaces/transactions_ledger.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`
