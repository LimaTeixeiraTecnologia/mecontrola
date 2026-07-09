# Tarefa 4.0: Retry transitório limitado + `IsTransient` + idempotência

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Recuperar falha transitória de escrita com retentativa idempotente limitada, sem o usuário
recomeçar, e garantir exatamente um lançamento por chave. Depende de 3.0 (o passo já retorna erro).
Ver ADR-003.

<requirements>
- RF-22: falha transitória gera até 2 retentativas automáticas idempotentes com backoff curto
  (< ~2s no mesmo turno) pela chave (`wamid`, `itemSeq`, operação), antes de desistir.
- RF-23: esgotadas as retentativas, o pending entry permanece retomável; a próxima confirmação
  reexecuta a escrita sem repetir a classificação de categoria.
- RF-24: reprocessar a mesma (`wamid`, `itemSeq`, operação) resulta em replay, nunca segundo lançamento.
- RF-25: transação criada mas ledger de idempotência falho ⇒ não reexecuta a escrita em
  processamento posterior da mesma chave (reconcile).
</requirements>

## Subtarefas

- [ ] 4.1 Implementar `IsTransient(err) bool` no consumidor (`internal/agents`, application) com
  `errors.Is`/`errors.As` (timeout, `context.DeadlineExceeded`, connection reset; default: false).
- [ ] 4.2 Aplicar loop de retry localizado (máx 2 tentativas, backoff ~100ms com jitter, teto <~2s,
  respeitando `ctx.Done()`) ao trecho de escrita em `executeWithIdempotency`/`executeDirectWrite`;
  NÃO usar `workflow.Retry` de nível `Step` nem aumentar `Engine.MaxAttempts` (ver ADR-003).
- [ ] 4.3 Garantir que erro permanente falha imediatamente (herda 3.0) sem consumir tentativas.
- [ ] 4.4 Validar replay/reconcile via `IdempotentWrite.Execute` (chave estável) e o sinal `reconciled`.

## Detalhes de Implementação

Ver ADR-003 (loop localizado, seletor `design-patterns` = `reject`) e techspec.md "Interfaces Chave".
Constante `maxWriteAttempts` (sem prefixo `_` — R5.26 [HARD]). Idempotência garantida pela dedup do
módulo `transactions` (reconciled) + `agents_write_ledger` (UNIQUE `wamid,item_seq,operation`).

## Critérios de Sucesso

- Falha transitória na 1ª tentativa ⇒ persiste 1x na 2ª; usuário recebe confirmação de sucesso.
- Reprocessar mesma chave ⇒ replay; nenhum segundo lançamento.
- Transação criada + ledger falho ⇒ não reexecuta escrita (reconciled).
- Retry respeita cancelamento de context; sem retry de erro permanente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — escrita idempotente no consumidor `internal/agents`, pending workflow e ledger.
- `domain-modeling-production` — outcome de idempotência (`created|reconciled|replay|usecase_error`) como tipo fechado.
- `design-patterns-mandatory` — gate executado (resultado reject): loop direto, sem padrão GoF (Strategy/Decorator).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/usecases/idempotent_write.go`, `register_attempt.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`
- `internal/agents/infrastructure/persistence/write_ledger_repository.go`
- `internal/platform/workflow/combinators.go` (referência; não alterar)
