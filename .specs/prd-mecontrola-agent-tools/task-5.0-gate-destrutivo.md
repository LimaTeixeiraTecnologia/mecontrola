# Tarefa 5.0: OperationKinds novos + gate destrutivo + 3 tools sensíveis

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender o enum fechado `OperationKind` em
`internal/agents/application/workflows/confirm_state.go` com `OpUpdateRecurrence`,
`OpDeleteRecurrence` e `OpUpdateCard` (mais `String`/`IsValid`/`ParseOperationKind`). Em
`destructive_confirm_workflow.go`, migrar `executeOperation` de `switch` para
`map[OperationKind]func(...)` (ADR-001), adicionar `executeUpdateRecurrence`/`executeDeleteRecurrence`/
`executeUpdateCard`, e estender `successMessage` e `BuildImpactNote` (`TargetKind`
`"recurring_template"`). Criar as tools `update_recurrence` (RF-16), `delete_recurrence` (RF-17) e
`update_card` (RF-18) seguindo o idioma de `edit_entry`/`delete_entry`: montam `ConfirmState`,
chamam `engine.Start`, retornam `needsConfirmation=true` sem efetivar. `update_card` só passa pelo
gate quando altera o dia de vencimento; edição só de apelido/banco executa direto via
`CardManager.UpdateCard` (D-02). Depende da Tarefa 2.0. Paralelizável com as Tarefas 3.0 e 4.0.

<requirements>
- RF-16, RF-17, RF-18, RF-22, RF-23, RF-26
- Dependência: Tarefa 2.0
- Paralelizável com Tarefas 3.0 e 4.0
- Reuso do workflow único `destructive-confirm` via novos `OperationKind` fechados + dispatch por mapa (ADR-001)
- Nenhuma efetivação sem confirmação humana explícita (RF-22, RF-26)
- Resume por merge-patch antes de qualquer parse (RF-23)
- Estados de fronteira como tipos fechados (`OperationKind`/`AwaitingApproval`/`RunStatus`) — nunca string solta
- `update_card` condicional: gate só quando muda o dia de vencimento; senão executa direto (D-02)
- Zero comentários em Go de produção; tools finas sem regra/SQL/branching (R-ADAPTER-001, R-AGENT-WF-001)
</requirements>

## Subtarefas

- [ ] 5.1 `OperationKind`s novos (`OpUpdateRecurrence`, `OpDeleteRecurrence`, `OpUpdateCard`) +
  `String`/`IsValid`/`ParseOperationKind`
- [ ] 5.2 Dispatch por mapa `map[OperationKind]func(...)` + executores
  `executeUpdateRecurrence`/`executeDeleteRecurrence`/`executeUpdateCard`
- [ ] 5.3 Impact notes e mensagens (`BuildImpactNote`/`successMessage`, `TargetKind`
  `"recurring_template"`)
- [ ] 5.4 3 tools sensíveis: `update_recurrence`, `delete_recurrence`, `update_card`
- [ ] 5.5 Testes (unitários do workflow + cada tool; integração do gate para as 3 operações)

## Detalhes de Implementação

Ver techspec.md desta pasta — seções **Estado de confirmação — novos OperationKind (fechados)**,
**Tools novas (15)** (linhas `update_recurrence`/`delete_recurrence`/`update_card`), **ADR-001**
(reuso do `destructive-confirm` via novos `OperationKind` + dispatch por mapa). Seguir o idioma
verificado de `edit_entry`/`delete_entry`. Não duplicar conteúdo da techspec/ADR — referenciar.

## Critérios de Sucesso

- Gate cobre: confirm→executa, cancel→no-op, ambíguo→reprompt único, TTL→expira
- Nenhuma efetivação sem confirmação explícita (RF-22, RF-26)
- Resume aplicado por merge-patch antes de qualquer parse (RF-23)
- Run conclui (`succeeded`/`failed`) sem suspenso órfão; estado de espera limpo deterministicamente
- Estados representados como tipos fechados (`OperationKind`/`AwaitingApproval`/`RunStatus`)
- `update_card` sem mudança de dia de vencimento executa direto via `CardManager.UpdateCard` (D-02)

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — tools de escrita e gate destrutivo montam primitivos do substrato internal/platform (tool, workflow) no molde internal/agents.

## Testes da Tarefa

- [ ] Testes unitários (confirm workflow + cada uma das 3 tools: confirm→executa, cancel→no-op,
  ambíguo→reprompt único, TTL→expira; tool retorna `needsConfirmation=true` sem efetivar)
- [ ] Testes de integração do gate para as 3 operações via `testcontainers` (`//go:build integration`):
  Start → suspend persistido → Resume → efetivação → run concluído

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/confirm_state.go`
- `internal/agents/application/workflows/destructive_confirm_workflow.go`
- `internal/agents/application/tools/update_recurrence.go`
- `internal/agents/application/tools/delete_recurrence.go`
- `internal/agents/application/tools/update_card.go`
