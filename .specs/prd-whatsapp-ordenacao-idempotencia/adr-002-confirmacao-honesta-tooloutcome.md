# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Confirmação honesta via propagação de `ToolOutcome` e guarda contra envio vazio
- **Data:** 2026-07-01
- **Status:** Aceita
- **Decisores:** time de plataforma (autor), owner do produto (decisão D-03 do PRD)
- **Relacionados:** PRD (RF-04/05/06/07/08), techspec `techspec.md`, ADR-001,
  regras R-AGENT-WF-001, R-ADAPTER-001, `domain-modeling.md` (DMMF), `error-handling.md`

## Contexto

O diagnóstico provou "sucesso alucinado": o agente respondeu "Despesa registrada com sucesso!" com
`transactions`/`budgets_expenses`/`agents_write_ledger` = 0. A pesquisa de código localizou o
mecanismo exato:

- `internal/platform/agent/agent.go` `invokeToolCall`: quando a tool retorna erro
  (`invokeErr != nil`), o código faz `content = ""` e devolve um tool message **vazio** ao LLM —
  **erro engolido** (viola `error-handling.md`: proibido engolir erro de persistência).
- `internal/platform/agent/runtime.go` `Execute`: hardcoda `RunStatusSucceeded` +
  `ToolOutcomeRouted` sempre que `execErr == nil`, mesmo com conteúdo vazio; o `Outcome` devolvido
  **não carrega** o `ToolOutcome`.
- `whatsapp_inbound_consumer.go` `sendReply`: com `content == ""` faz `return nil` — **não envia e
  não erra** (resposta vazia silenciosa).
- As write tools (`register_expense.go` etc.) já chamam `IdempotentWrite`, que retorna
  `IdempotentWriteResult{Outcome agent.ToolOutcome}` — mas o output da tool só expõe `IsReplay bool`,
  perdendo o outcome tipado.

`agent.ToolOutcome` já é um **tipo fechado** (`internal/platform/agent/types.go:48`: routed, clarify,
usecaseError, missingResolver, replay, reconciled) — DMMF state-as-type. A infraestrutura correta
existe; falta **propagá-la** e **não engolir** o erro.

## Decisão

Tornar a confirmação **derivada do resultado tipado**, ponta a ponta:

1. `invokeToolCall` passa a sinalizar falha de forma tipada (status fechado `toolExecOK|toolExecError`)
   e a entregar ao LLM um tool message com **erro estruturado** (não `content==""`), preservando o
   `%w` do erro. Nunca engolir.
2. `runtime.Execute` deriva `RunStatus`/`ToolOutcome` do resultado real (não hardcoda) e inclui
   `Outcome ToolOutcome` no `Outcome` retornado ao consumidor.
3. As write tools expõem o `ToolOutcome` no output tipado (não apenas `IsReplay`), de modo que
   `usecaseError`/`missingResolver` **nunca** virem confirmação de sucesso.
4. `sendReply` ganha **guarda contra envio vazio** (RF-08): `content==""` → fallback honesto
   ("não consegui concluir agora, pode repetir?") e **nunca** `SendTextMessage("")`; o resultado é
   observável (métrica `no_reply`), mas jamais um envio em branco.
5. Idempotência ligada **por padrão** (RF-04): remover o gate `AGENT_WRITE_ADVISORY_LOCK` **e deletar
   `advisory_key_locker.go`** (caminho `pg_advisory_lock` de sessão — redundante e inseguro sob
   pgbouncer transaction-pool; decisão travada). A serialização por usuário passa a ser garantida pelo
   claim particionado (ADR-001), e o `agents_write_ledger` (UNIQUE `(wamid,item_seq,operation)`)
   continua a fonte de verdade de replay.

## Alternativas Consideradas

1. **Releitura do `agents_write_ledger` antes de confirmar:** dupla checagem, +1 query por turno e
   acoplamento; rejeitada (D-03) — o resultado tipado da tool já é fonte suficiente.
2. **Manter erro engolido e apenas trocar texto vazio por fallback:** trata o sintoma (envio vazio)
   mas não a causa (LLM decide sem saber que a escrita falhou); rejeitada — não elimina alucinação.
3. **Result[T,E] custom / monada de erro:** proibido por DMMF (`domain-modeling.md`) e governança;
   rejeitada — usa-se `error` idiomático + tipo fechado `ToolOutcome`.

## Consequências

### Benefícios Esperados

- Fim do sucesso alucinado: confirmação só com persistência real → integridade do dado financeiro.
- Fim da mensagem vazia (RF-08): usuário sempre recebe feedback honesto.
- Idempotência sempre ativa → sem duplicidade em redelivery, sem depender de flag.

### Trade-offs e Custos

- Mudança em camada sensível (loop do agente + runtime); exige testes cuidadosos de não-regressão.
- O LLM passa a receber erros de tool; o prompt/policy deve orientar resposta honesta (sem inventar).

### Riscos e Mitigações

- **Risco:** regressão no loop de tool-calling. **Mitigação:** testes unitários por cenário
  (ok/replay/usecaseError/missingResolver/erro de IO) + validação com LLM real
  (memória `feedback_realllm_validation_required`: `RUN_REAL_LLM=1`). **Rollback:** reverter os
  4 pontos é isolado por função.
- **Risco:** fallback honesto confundido com erro sistêmico. **Mitigação:** métrica/outcome distinto.

## Plano de Implementação

1. `ToolOutcome` no `Outcome` do runtime + derivação de `RunStatus` do resultado real.
2. `invokeToolCall`: status tipado + tool message de erro estruturado (sem `content==""`).
3. Write tools: output carrega `ToolOutcome`.
4. `sendReply`: guarda de envio vazio + fallback honesto.
5. Remover gate `WriteAdvisoryLock` em `cmd/worker/worker.go` (idempotência default).
6. Testes unitários (testify/suite) + integração (erro de persistência → sem sucesso/sem vazio) +
   validação com LLM real.

Concluído quando: CA-02 e CA-03 verdes; taxa de outbound vazio = 0; divergência confirmação-vs-ledger = 0.

## Monitoramento e Validação

- Métricas: outbound vazio (=0), `send_error`, distribuição de `ToolOutcome`, duplicidade (=0).
- Alertar se qualquer confirmação de sucesso não tiver linha correspondente em `agents_write_ledger`.

## Impacto em Documentação e Operação

- Runbook do agente (semântica de outcome), prompt/policy (resposta honesta em falha), dashboards.

## Revisão Futura

- Revisar se novas write tools forem adicionadas (garantir que expõem `ToolOutcome`) ou se a política
  "lançamentos para relatório" mudar (memória `project_agent_writes_report_only`).
