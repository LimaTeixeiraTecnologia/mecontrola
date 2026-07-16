# Tarefa 4.0: Catálogo central de mensagens de tom de voz

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o catálogo central de mensagens determinísticas de tom de voz em `internal/agents/application/messages/`: funções puras que produzem os blocos verbatim do documento (confirmação de despesa/receita, sucesso de despesa/receita, resumo por categoria e geral, informacionais, esclarecimento, e as mensagens de estouro de orçamento). Elimina os geradores por workflow espalhados, estabelecendo fonte única testável consumida pelas tools/workflows e pelo guard `verbatim_relay`.

<requirements>
- RF-05: mensagens de tom de voz verbatim, fonte única.
- ADR-003: catálogo central de mensagens determinísticas; tom ancorado em duas fontes (prompt validado como base + blocos verbatim do documento como sobreposição normativa; documento prevalece em conflito).
- R-ADAPTER-001.1: zero comentários em `.go` de produção.
- R-AGENT-WF-001.4: sem LLM no caminho de decisão do catálogo (funções puras).
</requirements>

## Subtarefas

- [ ] 4.1 Criar o pacote `internal/agents/application/messages/` com funções PURAS (sem IO, sem `context.Context`, sem `time.Now`/random no caminho de decisão) que produzem os blocos verbatim do documento: confirmação de despesa (`✅ Encontrei este lançamento:` com 💰/💳/📂 + `Posso registrar?`), confirmação de receita (`✅ Encontrei esta entrada:` com 💰/📥 + `Posso registrar?`), sucesso de despesa (`Prontinho! ✅` + frase motivacional), sucesso de receita (`Boa notícia! 🎉` + frase motivacional), resumo por categoria e geral, informacionais (cancelamento Kiwify, suporte), esclarecimento, e as mensagens de "atingiu exatamente" / "ultrapassou em R$".
- [ ] 4.2 Rotação da frase motivacional determinística por seed estável (ex.: `messageID`), a partir de listas fixas por cenário — sem `time.Now`/random no caminho de decisão.
- [ ] 4.3 Ancorar o tom em DUAS fontes: a seção de Identidade/Tom de `internal/agents/application/agents/mecontrola_agent.go` como base, sobreposta pelos blocos verbatim do documento (documento prevalece em conflito).
- [ ] 4.4 Testes unitários comparando cada bloco produzido ao documento (verbatim) e verificando a rotação determinística por seed.

## Detalhes de Implementação

Ver `techspec.md` (RF-05) e `adr-003-message-catalog.md` desta pasta — **referenciar em vez de duplicar**.

Pontos-chave do ADR-003:
- Funções puras que produzem os blocos verbatim e sorteiam a frase motivacional por cenário a partir de listas fixas (rotação determinística por seed estável).
- As mensagens chegam ao usuário via `ResponseText` (resume) ou via guard `verbatim_relay` (fluxo LLM); o catálogo é a fonte única.
- Tom ancorado em duas fontes combinadas: seção Identidade/Tom de `mecontrola_agent.go` (base) + blocos verbatim do documento (sobreposição normativa); em conflito, o documento prevalece.
- Scorer de aderência (`verbatim_tone_adherence`) verifica conformidade — fora do escopo desta tarefa (task 9.0).

## Critérios de Sucesso

- Cada bloco produzido bate o documento verbatim (100% dos blocos).
- Rotação da frase motivacional é determinística para o mesmo seed.
- Funções são puras: sem IO, sem `context.Context`, sem `time.Now`/random no caminho de decisão.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — mensagens determinísticas consumidas por tools/workflows do agente e pelo guard verbatim.
- `design-patterns-mandatory` — gate de desenho do catálogo como fonte única.

## Testes da Tarefa

- [ ] Testes unitários (verbatim de cada bloco contra o documento + rotação determinística por seed)
- [ ] Testes de integração (não obrigatória nesta tarefa)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/messages/catalog.go` (+ tipos view/seed)
- `internal/agents/application/messages/catalog_test.go`
- `internal/agents/application/agents/mecontrola_agent.go` (fonte-base do tom; leitura)
