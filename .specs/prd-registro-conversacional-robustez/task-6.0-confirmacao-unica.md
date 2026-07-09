# Tarefa 6.0: Confirmação única — gate HITL é o dono; LLM nunca confirma

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Eliminar a confirmação dupla: o gate HITL do workflow é o único emissor de confirmação; o LLM apenas
repassa o `message` da tool e nunca formula pergunta de confirmação própria. Ver ADR-004.

<requirements>
- RF-14: o LLM não emite pergunta de confirmação; a confirmação é exclusiva do gate HITL do workflow.
- RF-15: o gate HITL emite um único resumo por lançamento ("Confirma? R$ X em Raiz > Folha para
  data no pagamento?").
- RF-16: após um único "sim", a escrita ocorre direto, sem segunda pergunta.
- RF-17: gate durável e idempotente (estado de espera fechado no snapshot antes de perguntar,
  retomada por merge-patch).
- RF-18: cancelamento explícito ("não") descarta sem gravar.
</requirements>

## Subtarefas

- [x] 6.1 Reescrever as instruções de confirmação em `mecontrola_agent.go` (`:58-66`, `:158-165`):
  remover a responsabilidade de "aguardar sim/não"/"exigir confirmação" do LLM; instruir a sempre
  chamar a tool de escrita imediatamente e repassar literalmente o `message` de `outcome=clarify`.
- [x] 6.2 Garantir que `buildConfirmSummary` (`pending_entry_workflow.go:712`) seja o único texto de
  confirmação; verificar os pontos de transição para `AwaitingSlotConfirmation` (`:156,:255,:335,:686`).
- [x] 6.3 Confirmar que "após confirmar, não chamar a ferramenta novamente" está explícito.

## Detalhes de Implementação

Ver ADR-004. O gate já é durável/idempotente (snapshot + merge-patch, replay-guard por
`ProcessedMessageID`). Nenhuma mudança no kernel; LLM só nas call-sites sancionadas.

## Critérios de Sucesso

- "Comprei um livro de R$ 50,00 no pix" → um único resumo → "sim" → grava em
  Conhecimento > Livros e E-books (pix), sem segunda confirmação.
- LLM não emite confirmação própria; "não" descarta sem gravar.
- Resumo único vem de `buildConfirmSummary` (path completo + data).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — instruções do agente, gate HITL durável e loop tool-calling no consumidor `internal/agents`.
- `domain-modeling-production` — estado de espera `AwaitingSlotConfirmation`/`ConfirmAction` como tipo fechado.
- `design-patterns-mandatory` — gate `não aplicar padrão` (ajuste de instruções, sem estrutura nova).

## Testes da Tarefa

- [x] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/mecontrola_agent.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`, `pending_entry_decisions.go`
