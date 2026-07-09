# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Idempotência e métrica de escrita do cartão via `IdempotentWriter` dos agents (`operation="create_card"`)
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Usuário (product owner), engenharia de plataforma agentiva
- **Relacionados:** PRD (RF-13, RF-14, RF-15, RF-16); techspec `techspec.md`; `internal/agents/application/usecases/idempotent_write.go`; `internal/agents/application/workflows/pending_entry_workflow.go` (`executeWithIdempotency`); `internal/card/application/usecases/create_card.go:80,115-132`

## Contexto

Existem dois mecanismos de idempotência no repositório:

1. **Interno do módulo card** — `idempotency.FromContext(ctx)` + `idem.Put` em
   `create_card.go:80,115-132`. Está **dormente** no caminho conversacional: a evidência de produção
   mostra 0 linhas em `idempotency_keys` para o cadastro, porque o único chamador (onboarding) não
   injeta contexto de idempotência.
2. **Dos agents** — `IdempotentWriter.Execute(ctx, userID, wamid, itemSeq, operation, resourceKind,
   write)` (`idempotent_write.go`), que envolve a escrita, grava `agents_write_ledger` para replay e
   emite a métrica `agents_write_total{operation,outcome}`. É o mecanismo usado por `register_expense`
   e pelo `pending_entry_workflow` (`executeWithIdempotency`). Hoje não há
   `agents_write_total{operation="create_card"}`.

O incidente exige (RF-13/RF-15) que toda tentativa seja run auditável com erro real persistido — nunca
falha silenciosa — e (RF-16) métrica de escrita com cardinalidade controlada.

## Decisão

Rotear a escrita do cartão pelo **`IdempotentWriter` dos agents**, dentro do step de execução do
workflow `card-create-confirm` (na resposta afirmativa "sim"):

```
idem.Execute(ctx, userID, wamid, itemSeq, "create_card", "card", writeFn)
```

onde `writeFn` invoca `CardManager.CreateCard(ctx, NewCard{...})` e retorna `(cardID uuid.UUID,
reconciled bool=false, err error)`.

- **Chave de idempotência:** `wamid` = `CardCreateState.MessageID` (wamid que iniciou a confirmação),
  `itemSeq = 0` (escrita única por cadastro). Replay do mesmo cadastro retorna a mesma resposta
  (`ToolOutcomeReplay`) sem criar segundo cartão (RF-14).
- **Métrica:** `agents_write_total{operation="create_card",outcome}` emitida automaticamente pelo
  `IdempotentWriter` (RF-16), sem `user_id` (R-AGENT-WF-001.5 / R-TXN-004).
- **Idempotência interna do módulo card permanece dormente** — NÃO injetar `idempotency.FromContext`
  no caminho conversacional; mecanismo único evita dupla persistência.
- **Erro real persistido (RF-15):** o continuer do workflow abre/fecha um `Run` auditável; em falha de
  infraestrutura, `closeRun(RunStatusFailed, err)` preenche a coluna de erro; o log estruturado
  `card.create.failed` do usecase permanece.
- **Distinção domínio vs infraestrutura:** conflito de apelido (`ErrNicknameConflict`) e validações
  (`ErrInvalidNickname`/`ErrInvalidDueDay`/`ErrInvalidClosingDay`/`ErrInvalidBank`) são **outcomes de
  domínio esperados** → mensagem acionável ao usuário + run concluído (não é falha silenciosa). Erros
  de infraestrutura → retry transiente (padrão `IsTransient`) e, persistindo, `RunStatusFailed` com
  erro na coluna do run + "tente novamente em breve".

## Alternativas Consideradas

- **Ativar a idempotência interna do módulo card + emitir métrica à parte.** Fiel ao texto literal de
  RN9. Desvantagem: mantém dois sistemas de idempotência (card `idempotency_keys` + agents
  `agents_write_ledger`), duplica instrumentação e diverge do padrão `register_expense`/`pending_entry`.
  Rejeitada.
- **Sem idempotência, confiar só no índice único de apelido.** O índice previne duplicata por apelido,
  mas não dá resposta de replay estável nem métrica; não cobre corrida de dois "sim" concorrentes antes
  da conclusão do run. Rejeitada.

## Consequências

### Benefícios Esperados

- RF-14 e RF-16 atendidos por um único mecanismo idiomático, consistente com `register_expense`.
- `agents_write_ledger` e `agents_write_total` passam a cobrir `create_card` (observabilidade paritária).
- Replay estável keyed por `wamid`.

### Trade-offs e Custos

- O `BuildCardCreateConfirmWorkflow` ganha dependência de `IdempotentWriter` (além de `CardManager`).
- A idempotência interna do card segue dormente (código morto tolerado; sua ativação é outro escopo).

### Riscos e Mitigações

- **Risco:** `writeFn` precisa converter `CardRef.ID string` → `uuid.UUID`. **Mitigação:** parse com
  erro tratado (`%w`); ID é UUID canônico do card.
- **Risco:** contagem de `outcome` divergir da semântica de cartão. **Mitigação:** mapear
  `ErrNicknameConflict` para outcome de domínio (não `usecaseError` de infra) na fronteira do workflow.

## Plano de Implementação

1. `BuildCardCreateConfirmWorkflow(idem interfaces.IdempotentWriter, cards interfaces.CardManager)`.
2. `executeCreateCard` monta `writeFn` e chama `idem.Execute(..., "create_card", "card", writeFn)`.
3. Continuer abre/fecha `Run`; mapeia outcomes para mensagem acionável (domínio) ou falha (infra).

## Monitoramento e Validação

- Alerta se `agents_write_total{operation="create_card",outcome="usecase_error"}` subir anormalmente.
- Verificar em produção `agents_write_ledger` e `agents_write_total{operation="create_card"}` após deploy.
- Teste de replay: segundo "sim" (mesmo wamid) não cria segundo cartão e retorna mesma resposta (RF-14).

## Impacto em Documentação e Operação

- Dashboards e runbook do agente: incluir `operation="create_card"` nas métricas de escrita.

## Revisão Futura

- Reavaliar consolidação dos dois mecanismos de idempotência (card interno vs agents) se o cadastro
  por app/HTTP passar a exigir o mecanismo interno ativo.
