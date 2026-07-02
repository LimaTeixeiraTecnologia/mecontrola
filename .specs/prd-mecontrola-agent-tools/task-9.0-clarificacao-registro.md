# Tarefa 9.0: Clarificação de registro (categoria/data) via ConfirmState não-destrutivo

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reintroduzir o fluxo de clarificação de registro de lançamentos, corrigindo o atrito inconsistente
comprovado em produção (EP-04: no primeiro registro o agente pediu categoria e confirmação de data; em
outro registrou "instantâneo" sem categoria). O agente deve perguntar a **categoria apenas quando
ausente/ambígua** (não resolvida com confiança por `classify_category`) e resolver a **data por default
determinístico** (data corrente em `America/Sao_Paulo`, inferindo "ontem"/data relativa/data explícita)
**sem perguntar**. O estado de espera reutiliza o substrato `ConfirmState`
(`internal/agents/application/workflows/confirm_state.go`) com um `OperationKind` **não-destrutivo**
dedicado (`OpConfirmRegister`), dispatch por mapa — sem criar mecanismo HITL paralelo (RF-43).

Depende da Tarefa 0.0 (substrato confiável — sem ela a confirmação de sucesso continua alucinada) e da
Tarefa 1.0 (contratos/tipos agent-owned). Convive com a Tarefa 5.0 (que estende o mesmo enum
`OperationKind` com kinds destrutivos): o ponto de convergência é `confirm_state.go`/
`destructive_confirm_workflow.go`, coordenado pela ordem do DAG.

<requirements>
- RF-41 — Clarificar a **categoria** antes de gravar **apenas quando** ausente ou ambígua; quando
  resolvida com confiança por `classify_category`, gravar sem perguntar (RF-21 — pede só o dado faltante).
- RF-42 — Resolver a **data** por default determinístico (data corrente em `America/Sao_Paulo`;
  inferir "ontem"/data relativa/data explícita) **sem perguntar**; confirmação de data só quando
  genuinamente ambígua.
- RF-43 — Estado de espera reutiliza `ConfirmState` com `OperationKind` **não-destrutivo**
  (`OpConfirmRegister`), respeitando o contrato de pending step (R-AGENT-WF-001.7): persistir o estado
  antes de perguntar, retomar por merge-patch antes de qualquer parse, concluir o Run
  deterministicamente. PROIBIDO criar mecanismo HITL paralelo ao existente.
- Dependência: Tarefa 0.0 (substrato) e Tarefa 1.0 (contratos).
- RTA-03 — `OpConfirmRegister` enumerado no mesmo tipo fechado `OperationKind`, sem string solta;
  `String()`/`IsValid()`/`ParseOperationKind()` atualizados.
- RTA-01 — resolução por mapa (`map[OperationKind]func(...)`), nunca `switch case intent.Kind`.
- Zero comentários em Go de produção (R-ADAPTER-001.1); sem abstração de tempo (`time.Now().UTC()` inline
  para o default de data em `America/Sao_Paulo`).
</requirements>

## Subtarefas

- [ ] 9.1 Estender o enum fechado `OperationKind` (`confirm_state.go`) com `OpConfirmRegister`
  **não-destrutivo** + `String()`/`IsValid()`/`ParseOperationKind()`.
- [ ] 9.2 Registrar `OpConfirmRegister` no dispatch por mapa (`map[OperationKind]func(...)`) do fluxo de
  confirmação, com o executor que efetiva o registro (delegando à tool/binding de escrita já corrigida
  pela Tarefa 0.0), distinto da semântica destrutiva.
- [ ] 9.3 Fluxo de clarificação de categoria: perguntar **apenas** quando `classify_category` não
  resolve com confiança (ausente/ambígua); quando resolve, seguir direto ao registro sem perguntar.
- [ ] 9.4 Resolução determinística de data (RF-42): default = data corrente `America/Sao_Paulo`;
  inferência de "ontem"/data relativa/data explícita; confirmação de data só em ambiguidade genuína.
- [ ] 9.5 Testes: categoria resolvida → grava sem perguntar; categoria ausente/ambígua → pergunta uma
  vez, persiste estado, retoma por merge-patch antes do parse; data default sem pergunta; conclusão
  determinística do Run (sem draft órfão).

## Detalhes de Implementação

Ver `prd.md` seções `Clarificação de registro (categoria/data)` (RF-41..RF-43), `Experiência do
Usuário` e `Evidência de Produção` (EP-04); decisão D-09; `techspec.md` seção "Estado de confirmação —
novos OperationKind (fechados)". Reutilizar integralmente o contrato de `ConfirmState` já endurecido
(persistir antes de perguntar, resume por merge-patch antes do parse, TTL, conclusão determinística) —
o `OpConfirmRegister` é apenas mais uma constante no mesmo tipo fechado, com semântica **não-destrutiva**
(sem nota de impacto/aviso destrutivo). A confirmação de sucesso de escrita depende da Tarefa 0.0
(guard anti-simulação): só confirmar ao usuário quando a tool de escrita retornar `ToolOutcome` real de
sucesso. Não duplicar conteúdo do PRD/techspec — referenciar. Trechos ilustrativos, se houver, com zero
comentários.

## Critérios de Sucesso

- Categoria resolvida com confiança por `classify_category` → grava sem perguntar (RF-41/RF-21).
- Categoria ausente/ambígua → pergunta uma vez, com `ConfirmState`/`OpConfirmRegister` persistido antes
  da pergunta e retomado por merge-patch antes de qualquer parse (RF-43).
- Data resolvida por default determinístico sem perguntar; confirmação só em ambiguidade genuína (RF-42).
- Nenhum mecanismo HITL paralelo criado; `OpConfirmRegister` é constante do mesmo `OperationKind` fechado
  (RF-43/RTA-03), resolvido por mapa (RTA-01).
- Run conclui deterministicamente (sem draft/suspenso órfão); sucesso de escrita só confirmado com
  `ToolOutcome` real (integra Tarefa 0.0).
- Zero comentários em Go de produção; sem abstração de tempo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — reuso do pending step/`ConfirmState` e do gate de confirmação segue o molde
  `internal/agents` sobre `internal/platform`; estado de espera fechado, resume por merge-patch antes do
  parse (R-AGENT-WF-001.7).

## Testes da Tarefa

- [ ] Testes unitários — categoria resolvida→grava sem perguntar; ausente/ambígua→pergunta única com
  estado persistido; data default sem pergunta; `OpConfirmRegister` como tipo fechado
  (`String`/`IsValid`/`ParseOperationKind`); resume por merge-patch antes do parse.
- [ ] Testes de integração — fluxo de clarificação end-to-end via `testcontainers`
  (`//go:build integration`): Start → suspend persistido → Resume por merge-patch → registro efetivado →
  run concluído; escrita real assertada no banco (integra Tarefa 0.0/RF-29).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/confirm_state.go`
- `internal/agents/application/workflows/destructive_confirm_workflow.go` (dispatch por mapa)
- `internal/agents/application/tools/register_expense.go` / `register_income.go` (efetivação do registro)
- `internal/agents/application/agents/mecontrola_agent.go` (instrução: categoria só quando ausente/ambígua)
