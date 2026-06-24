# Tarefa 4.0: Passos HITL — prepare_target, confirm_gate, execute_destructive

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os três passos novos do workflow de confirmação, reusando os passos de guarda existentes
(`authorize`, `replay`, `policy`, `audit_begin`). `prepare_target` resolve o alvo e compõe o prompt;
`confirm_gate` suspende/retoma a aprovação humana (semântica estrita + TTL + re-prompt único);
`execute_destructive` efetiva a mutação despachando por `OperationKind` para o binding existente.

<requirements>
- RF-08: as 4 operações destrutivas exigem confirmação humana explícita antes de efetivar.
- RF-11: estado de espera fechado (`AwaitingApproval`); semântica determinística sem LLM.
- RF-12: intervenção humana auditável (reuso de `audit_begin`/`OnSettle`/decision-id).
- RF-13: resume idempotente; limpeza determinística após efetivar/cancelar/expirar.
</requirements>

## Subtarefas

- [ ] 4.1 `prepare_target.go`: `map[OperationKind]TargetResolver` (delete/edit último, deletar cartão, budget commit); alvo inexistente → short-circuit com mensagem. Adapter fino sobre bindings.
- [ ] 4.2 `confirm_gate.go`: 1ª passada seta `AwaitingConfirm` e suspende com prompt; no resume interpreta confirma/cancela (matchers determinísticos) / ambíguo-1 (re-prompt, `RepromptCount=1`) / ambíguo-2 (cancela) / expirado (`SuspendedAt` > TTL → cancela sem efeito).
- [ ] 4.3 `execute_destructive.go`: `map[OperationKind]DestructiveExecutor` chamando os usecases via bindings existentes; sem regra/SQL/branching de domínio.
- [ ] 4.4 Reusar `NewAuthorize/NewReplay/NewPolicy/NewAuditBegin/NewFormat` (sem duplicar guarda).

## Detalhes de Implementação

Ver `techspec.md` seções "Design de Implementação" (assinaturas `TargetResolver`/`DestructiveExecutor`)
e ADR-003 (transições do gate) + ADR-004 (budget no commit). Tempo via `time.Now().UTC()` inline (sem
abstração de clock). Zero comentários. `context.Context` em toda fronteira de IO.

## Critérios de Sucesso

- `confirm_gate` cobre os 5 caminhos (confirma/cancela/ambíguo-1/ambíguo-2/expira) deterministicamente.
- `prepare_target`/`execute_destructive` despacham por `OperationKind` via mapa (sem `switch` de domínio).
- Executores são adapters finos: sem regra de negócio, SQL ou branching de domínio.
- Nenhum LLM invocado em qualquer passo (R-AGENT-WF-001.4).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — cada caminho do `confirm_gate`; `prepare_target` alvo inexistente → short-circuit; cada `OperationKind` mapeado ao executor correto (mock do binding).
- [ ] Testes de integração — não aplicável nesta tarefa (coberto em 7.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/workflow/steps/prepare_target.go` (novo)
- `internal/agent/application/workflow/steps/confirm_gate.go` (novo)
- `internal/agent/application/workflow/steps/execute_destructive.go` (novo)
- `internal/agent/application/workflow/steps/steps_test.go` (estendido)
- `internal/agent/application/workflow/steps/{authorize,replay,policy,audit_begin,format}.go` (reuso)
