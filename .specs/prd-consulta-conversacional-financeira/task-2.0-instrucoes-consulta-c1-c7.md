# Tarefa 2.0: Bloco de instruções C1–C7 na const `mecontrolaAgentInstructions`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Evoluir a constante de instruções `mecontrolaAgentInstructions`
(`internal/agents/application/agents/mecontrola_agent.go:15-153`) com um bloco declarativo de
"Consultas Financeiras (C1–C7)": matriz de roteamento determinístico, regras de formatação de valores,
mapa slug→nome, exibição de alertas, retrocesso de mês, anti-alucinação, ambiguidade de cartão, guard
de `cardId`, mensagens de erro e formatação WhatsApp. É **apêndice de seção** — não reescrever as
regras existentes de registro/edição/confirmação/HITL. O roteamento é resolvido pelo loop de
tool-calling (ADR-001); proibido `switch case intent.Kind` (R-AGENT-WF-001.1).

<requirements>
- RF-01..RF-05, RF-06, RF-07, RF-08, RF-09: matriz de roteamento C1–C7 (ver tabela na techspec).
- RF-06a: enriquecimento de categoria em C5 é best-effort (falha → desc/valor/data, sem erro).
- RF-07a: mês atual vazio em C5/C6 → 1 retrocesso via `query_month` do mês anterior; senão RF-30.
- RF-08a: C2/C3/C7 sempre resumem alertas ativos (ou informam ausência) usando o array `alerts`.
- RF-10..RF-12: anti-alucinação e recusa de fora de domínio.
- RF-13/RF-14: competência `YYYY-MM`; "mês atual" via `America/Sao_Paulo`.
- RF-15/RF-16/RF-17: ambiguidade de cartão (`resolve_card.found=false`→`list_cards`); `limit=5` default; mês atual default.
- RF-18..RF-22: C7 todas as categorias (nome via mapa, planejado/gasto/percentual, `plannedCents` nulo → "Sem limite definido", total no topo, R$ com 2 casas e milhar).
- RF-19 (D-02): mapa fixo slug→nome das 5 raízes.
- RF-23..RF-25: PT-BR, emojis 📊/💰/✅, negrito só `*simples*`, nunca `**`.
- RF-26/RF-27: memória de thread para follow-up sem substituir chamada de tool.
- RF-28..RF-31: mensagens de erro/ausência verbatim.
- RF-32/RF-32a (D-08): responder só sobre o `resourceID` da thread; `cardId` só de `resolve_card`/`list_cards`.
- RF-33/RF-34: read-only idempotente; ordenação `createdAt` desc (garantida pela origem; exceção retrocesso RF-07a).
- RF-35: escopo — só instruções (mais o campo aditivo da 1.0); não tocar `module.go`/bindings/use cases.
- RF-36 (ADR-003): regra de formatação única e canônica no prompt (sem presenter Go).
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar a matriz de roteamento C1–C7 (gatilhos → tools) como nova seção da const.
- [ ] 2.2 Adicionar regra canônica de formatação de valores (ex.: `123450`→`R$ 1.234,50`) e o mapa slug→nome das 5 raízes.
- [ ] 2.3 Especificar C5 (categoria/subcategoria + best-effort), C6 (lista sem categoria por item), retrocesso de mês (RF-07a) e exibição de alertas em C2/C3/C7 (RF-08a).
- [ ] 2.4 Especificar competência/fuso, ambiguidade de cartão, guard de `cardId`, anti-alucinação, recusa de domínio, mensagens de erro verbatim e formatação WhatsApp.
- [ ] 2.5 Garantir que as regras existentes (registro/edição/confirmação/HITL) permanecem intactas (diff apenas aditivo na const).

## Detalhes de Implementação

Ver techspec.md, seção "Protocolo de Instruções (bloco C1–C7)" (matriz + regras) e ADR-001/ADR-003.
Usar os diálogos C1–C7 do PRD como exemplos canônicos verbatim no prompt para ancorar formato.

## Critérios de Sucesso

- Bloco C1–C7 presente na const, cobrindo todas as regras dos `<requirements>`.
- Regras existentes preservadas (nenhuma remoção/reescrita de confirmação/escrita).
- Zero `switch case intent.Kind`; roteamento continua pelo loop tool-calling.
- `go build`/`go vet ./internal/agents/...` verdes; `golangci-lint` sem novos achados; zero comentários.
- `mecontrola_agent_test.go` (builder: ID/instructions/tools) verde; suíte `pending_entry_*` verde (não-regressão).
- Nenhuma mudança em `module.go`, bindings ou use cases.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — evolução das instruções do agente `mecontrola` (roteamento por loop tool-calling, sem switch de intent; substrato `internal/platform/agent`).

## Testes da Tarefa

- [ ] Testes unitários (`mecontrola_agent_test.go` do builder; não-regressão da suíte de agents)
- [ ] Testes de integração (comportamento C1–C7 validado no gate real-LLM da Tarefa 3.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/mecontrola_agent.go` (const `mecontrolaAgentInstructions` — modificado)
- `internal/agents/application/agents/mecontrola_agent_test.go` (builder — referência/possível ajuste)
- Dependência: Tarefa 1.0 (campo `subcategoryNameSnapshot` disponível para C5)
