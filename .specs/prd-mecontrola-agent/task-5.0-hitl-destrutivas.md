# Tarefa 5.0: HITL de operações destrutivas (ConfirmState fechado, resume antes do parse)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o gate Human-in-the-Loop para operações destrutivas (remover/editar lançamento, remover compra parcelada, remover cartão), reemitindo o contrato `R-AGENT-WF-001.7-A` para o consumidor `internal/agents`. O domínio não oferece proteção; o agente confirma com aviso de impacto antes de efetivar.

<requirements>
- ADR-005: `AwaitingKind`/`OperationKind` fechados; `ConfirmState` persistido no `Snapshot` do kernel (sem side-store); resume antes do parse; re-prompt único; limpeza determinística; sem LLM no passo de confirmação.
- Aviso de impacto: parcelado remove todas as parcelas; cartão alerta órfãos (`HasOpenInstallments`).
- Cobre: RF-27.
</requirements>

## Subtarefas

- [ ] 5.1 Tipos fechados `AwaitingKind` (`AwaitingNone`/`AwaitingConfirm`), `OperationKind` (`OpDeleteEntry`/`OpEditEntry`/`OpDeleteCard`) e `ConfirmState` com smart constructors.
- [ ] 5.2 Persistir `ConfirmState{Awaiting:AwaitingConfirm}` no `Snapshot` **antes** de retornar a pergunta de confirmação.
- [ ] 5.3 `continueDestructiveConfirm`: rodar **antes de qualquer parse**; merge-patch (`{"ResumeText":"sim"}`); confirma→executa via binding; cancela→descarta; ambíguo→re-pergunta uma vez (`RepromptDone`) depois cancela; replay de `messageID` → `ToolOutcomeReplay`.
- [ ] 5.4 Montar `ImpactNote` (parcelas/órfãos via `HasOpenInstallments`).
- [ ] 5.5 Limpeza determinística: run nunca permanece suspenso após efetivar/cancelar.

## Detalhes de Implementação

Ver techspec.md → "ConfirmState" e ADR-005. Estado no `Snapshot` do kernel (R-WF-KERNEL-001 / R-AGENT-WF-001.7). LLM proibido no `confirm_gate` (R-AGENT-WF-001.4).

## Critérios de Sucesso

- `AwaitingKind`/`OperationKind` nunca como string livre (DMMF state-as-type; gate de governança verde).
- Resume ocorre **antes** do parse (ordem determinística); teste de guarda.
- Nenhuma operação destrutiva efetiva sem "sim" explícito; aviso de impacto presente.
- Snapshot é fonte única (sem side-store de domínio); limpeza sem run órfão.
- Zero comentários em `.go`; build/gofmt/governança verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — estado de espera (pending step / confirmação) no ciclo do agente sobre o kernel de workflow; contrato HITL do substrato.

## Testes da Tarefa

- [ ] Testes unitários: confirma/cancela/ambíguo(re-prompt único)/replay/limpeza; resume antes do parse.
- [ ] Testes de integração (testcontainers): `ConfirmState` persistido/retomado no `Snapshot`; nenhum run destrutivo permanece suspenso.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/` (confirmação destrutiva)
- Depende das tools `edit_entry`/`delete_entry` e `CardManager.SoftDeleteCard`/`HasOpenInstallments` (2.0/4.0)
- techspec.md (ConfirmState), ADR-005; `.claude/rules/agent-workflows-tools.md` (Addendum .7-A)
