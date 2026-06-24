# Tarefa 8.0: [Fase 2 — adiada] Plano multi-tool determinístico (capacidade A)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

**Guarda-chuva de roadmap — NÃO implementar neste MVP.** Esta tarefa rastreia a capacidade A do PRD
(plano determinístico multi-tool) como PLANEJADA-NÃO-IMPLEMENTADA. O MVP corrente entrega apenas a
capacidade B (HITL). Quando priorizada, esta tarefa deve ser **decomposta em uma rodada própria de
`create-tasks`** a partir das decisões já registradas no PRD e na seção "Fases Posteriores" da
techspec.

<requirements>
- RF-01: `ParseInbound` pode produzir plano ordenado de 1..N intents (1 item = comportamento atual).
- RF-02: execução do plano determinística — sem LLM durante os passos.
- RF-03: cada passo roda pelos IntentWorkflows/Tools existentes; sem novo `case intent.Kind`.
- RF-04: short-circuit em falha dura de escrita + agregação determinística das respostas.
- RF-05: resultado/outcome por passo auditável; sem user_id/category_id como label.
- RF-06: condição de parada como regra pura (sem avaliação por LLM).
- RF-07: comportamento single-intent permanece idêntico (não regressão).
</requirements>

## Subtarefas

- [ ] 8.1 [Adiada] Decompor a capacidade A em rodada própria de `create-tasks` (representação `Plan=[]Intent` aditiva; executor stop-on-first-write-failure).

## Detalhes de Implementação

Ver `prd.md` (RF-01..07) e `techspec.md` seção "Fases Posteriores — Fase A". Nenhum código nesta
tarefa. A decisão de produto já está fixada: `Plan=[]Intent` aditivo (mantém `intent.Intent` single),
executor determinístico com short-circuit na 1ª falha de escrita.

## Critérios de Sucesso

- Esta tarefa permanece `pending` até ser priorizada e decomposta.
- Nenhuma alteração de código de produção enquanto adiada.
- Ao priorizar: gerar tasks próprias herdando os gates `R-*` e a fronteira kernel-vs-agent.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — não aplicável enquanto adiada.
- [ ] Testes de integração — não aplicável enquanto adiada.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.specs/prd-agent-platform-evolution/prd.md` (RF-01..07)
- `.specs/prd-agent-platform-evolution/techspec.md` (seção "Fases Posteriores — Fase A")
