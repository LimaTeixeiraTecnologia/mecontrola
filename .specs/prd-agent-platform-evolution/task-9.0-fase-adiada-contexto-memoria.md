# Tarefa 9.0: [Fase 3 — adiada] Recuperação contextual estruturada + memória (capacidade C)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

**Guarda-chuva de roadmap — NÃO implementar neste MVP.** Esta tarefa rastreia a capacidade C do PRD
(recuperação contextual estruturada + memória) como PLANEJADA-NÃO-IMPLEMENTADA. Quando priorizada,
deve ser **decomposta em rodada própria de `create-tasks`** a partir das decisões já registradas no
PRD e na seção "Fases Posteriores" da techspec.

<requirements>
- RF-15: recuperação por query estruturada no Postgres existente — sem RAG vetorial/pgvector.
- RF-16: expor ao ContextBuilder histórico/padrões do usuário + taxonomia de categorias.
- RF-17: conteúdo recuperado entra no system prompt do ParseInbound; LLM só no parse.
- RF-18: resumo de histórico conversacional longo (pipeline assíncrono existente).
- RF-19: memória observacional estruturada e versionada (gatilho assíncrono mantido).
- RF-20: recuperação/memória escopo `resource` (por user_id), isoladas por usuário.
</requirements>

## Subtarefas

- [ ] 9.1 [Adiada] Decompor a capacidade C em rodada própria de `create-tasks` (retrieval estruturado + resumo + memória estruturada/versionada).

## Detalhes de Implementação

Ver `prd.md` (RF-15..20) e `techspec.md` seção "Fases Posteriores — Fase C". Nenhum código nesta
tarefa. Decisões de produto fixadas: fontes = histórico do usuário + taxonomia de categorias;
infraestrutura = query estruturada no Postgres existente (sem vetorial).

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
- `.specs/prd-agent-platform-evolution/prd.md` (RF-15..20)
- `.specs/prd-agent-platform-evolution/techspec.md` (seção "Fases Posteriores — Fase C")
