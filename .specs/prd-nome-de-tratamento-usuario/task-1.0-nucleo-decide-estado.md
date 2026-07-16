# Tarefa 1.0: Núcleo puro do fluxo de edição: estado fechado e funções Decide

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o núcleo puro (sem IO) do fluxo de alteração do nome de tratamento: o tipo fechado de ciclo de vida e as funções de decisão puras, com suíte de teste canônica. Base para os componentes de workflow e onboarding.

<requirements>
RF-02 (extração/validação do nome de tratamento), RF-11 (normalização + limite de 40 caracteres, rejeição de vazio/recusa).
</requirements>

## Subtarefas

- [x] 1.1 Criar `internal/agents/application/workflows/treatment_name_edit_state.go` com o tipo fechado `TreatmentNameEditStatus` (constantes `TreatmentNameEditActive`=iota+1, `TreatmentNameEditCompleted`, `TreatmentNameEditCancelled`, `TreatmentNameEditExpired`) + `String()`/`IsValid()`/`ParseTreatmentNameEditStatus` com sentinel error; e o struct `TreatmentNameEditState` (campos e tags JSON conforme techspec seção Modelos de Dados).
- [x] 1.2 Criar `internal/agents/application/workflows/treatment_name_edit_decisions.go` com `DecideTreatmentName(hasName bool, raw string) (string, bool)` (trim, rejeita vazio/!hasName, rejeita > `treatmentNameMaxLen`=40 via `utf8.RuneCountInString`) e `DecideTreatmentNameEditExpiry(state, now time.Time) bool` (usa `treatmentNameEditTTL`=15min); ambas puras (sem ctx, sem IO). `now` injetado (proibido abstrair tempo).
- [x] 1.3 Suíte canônica testify (whitebox `package workflows`) cobrindo: nome direto; vazio/recusa→(",false); >40→(",false); trim; expiry zero/dentro/fora da TTL.

## Detalhes de Implementação

Ver `techspec.md` seção "Modelos de Dados" e "Design de Implementação"; blueprint `internal/agents/application/workflows/goal_edit_state.go` (tipo fechado) e `goal_edit_decisions.go:53-59` (Decide puro). ADR-002.

## Critérios de Sucesso

tipo fechado com String/IsValid/Parse + sentinel; funções puras determinísticas; suíte verde; zero comentários; sem prefixo `_`; sem abstração de tempo (`now` injetado).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — Decide* puro e estado como tipo fechado (DMMF state-as-type) para o ciclo de vida da edição.
- `design-patterns-mandatory` — gate de desenho para o tipo fechado e as funções de decisão puras.

## Testes da Tarefa

- [ ] Testes unitários (suíte canônica das Decide* e do tipo)
- [ ] Testes de integração: não aplicável nesta tarefa

<critical>SEMPRE CRIAR E EXECUTAR TESTES antes de marcar a tarefa como concluída</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/treatment_name_edit_state.go` (novo)
- `internal/agents/application/workflows/treatment_name_edit_decisions.go` (novo)
- `internal/agents/application/workflows/treatment_name_edit_decisions_test.go` (novo)
- referência `internal/agents/application/workflows/goal_edit_state.go`
- referência `internal/agents/application/workflows/goal_edit_decisions.go`
