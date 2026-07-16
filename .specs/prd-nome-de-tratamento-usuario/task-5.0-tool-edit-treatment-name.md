# Tarefa 5.0: Tool fina edit_treatment_name delegando ao workflow

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar a tool adapter fina que o agente diário chama ao detectar a intenção de troca de nome; ela apenas mapeia o input tipado e dá `Start` no engine do workflow treatment-name-edit. Input carrega o nome opcional para satisfazer o turno único (RF-07).

<requirements>
- RF-06: intenção de troca roteada por tool.
- RF-07: nome no input → turno único.
</requirements>

## Subtarefas

- [ ] 5.1 Criar `internal/agents/application/tools/edit_treatment_name.go`: `EditTreatmentNameInput{ Name string `json:"name"` }` com `Validate() error` (nome opcional; sem IO — R-DTO-VALIDATE-001); `EditTreatmentNameOutput{ Status, Message string }`; `BuildEditTreatmentNameTool(engine workflow.Engine[workflows.TreatmentNameEditState], def workflow.Definition[workflows.TreatmentNameEditState]) tool.ToolHandle` via `tool.NewTool[...]`; `exec` puxa `agent.InboundRequest` de `wf.RuntimeFrom(ctx)`, monta o estado inicial (`ResourceID`, `MessageID`, `ProvidedName=in.Name`), computa `TreatmentNameEditKey(req.ResourceID, req.ThreadID)`, chama `engine.Start`; mapeia `ErrRunAlreadyExists`→"pending_exists"; sucesso→status/Message (ResponseText). Adapter fino: sem regra/SQL/branching de domínio.
- [ ] 5.2 Testes da tool (fake engine): started com nome; started sem nome (suspende); pending_exists.

## Detalhes de Implementação

Ver `techspec.md` (Interfaces Chave) e ADR-002. Blueprint EXATO: `internal/agents/application/tools/edit_goal.go:27-89` (porém input vazio; aqui o input carrega `name`). `wf.RuntimeFrom` e `tool.NewTool` conforme edit_goal.

## Critérios de Sucesso

Tool fina sem regra/SQL/branching (R-AGENT-WF-001.2/R-ADAPTER-001); input com `Validate()`; delega a `engine.Start`; zero comentários; testes verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — tool fina (adapter) delegando ao engine do workflow, sem regra/SQL/branching.
- `design-patterns-mandatory` — Adapter fino e validação de input DTO.

## Testes da Tarefa

- [ ] Testes unitários (tool com fake engine)
- [ ] Testes de integração (coberta na Tarefa 7.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/tools/edit_treatment_name.go` (novo) + teste
- Consome workflow da Tarefa 3.0
- Referência: `internal/agents/application/tools/edit_goal.go:27-89`
