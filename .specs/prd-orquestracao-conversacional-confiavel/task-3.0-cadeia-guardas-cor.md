# Tarefa 3.0: Cadeia de guardas conversacionais (Chain of Responsibility)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar as regras críticas hoje presas no prompt como uma cadeia de guardas conversacionais no
padrão Chain of Responsibility (confirmado pelo seletor de `design-patterns-mandatory`), como decorator
`agent.Agent` no consumidor `internal/agents`, absorvendo o `MultiItemGuard` atual e preservando
`BuildMeControlaAgent`.

<requirements>
- RF-01: cadeia CoR de handlers pequenos, ordenados, observáveis e testáveis, antes e depois do LLM,
  preservando `BuildMeControlaAgent` e a composição do módulo.
- RF-02: absorver o `MultiItemGuard` como primeiro `PreGuard`, preservando comportamento determinístico,
  `ToolOutcomeClarify` e a mensagem verbatim (teste de equivalência).
- RF-03: cada regra crítica vira guarda/roteador/validação com teste; a instrução vira reforço.
- RF-04/RF-05: bloquear múltiplos lançamentos antes do LLM (sem tool de escrita, outcome `clarify`);
  reconhecer padrão brasileiro `R$ 1.234,56` (não dispara multi-item).
- RF-06: cada handler expõe decisão observável (passou/tratou/delegou).
- RF-09: nunca afirmar sucesso/valor/categoria/status sem retorno real de tool — `PostGuard`
  `success_without_tool`.
- RF-10: resposta natural WhatsApp sem termos internos — `PostGuard` `internal_terms` (blocklist fechada).
- RF-11: preservar fluxo `adapter → tool → usecase`; guardas não contêm SQL/regra de domínio.
- RF-12: relay verbatim do `message`/`clarifyPrompt`/`confirmationPrompt` da tool — `PostGuard`
  `verbatim_relay`.
- RF-48: economia de LLM — `PreGuard` curto-circuita sem chamar o modelo.
</requirements>

## Subtarefas

- [x] 3.1 `guard_chain.go`: `GuardDecision`, `PreGuard`, `PostGuard`, `guardChainAgent` (embed
  `agent.Agent`, override `Execute`, delega `Stream`), `WithGuardChain`, métrica por handler.
- [x] 3.2 `guards/multi_item.go`: mover o `MultiItemGuard` para `PreGuard` (equivalência de saída).
- [x] 3.3 `guards/verbatim_relay.go` (`PostGuard`): força `Content` = `verbatimText` da tool quando
  divergir.
- [x] 3.4 `guards/empty_answer.go` (`PostGuard`): `Content` vazio → fallback seguro.
- [x] 3.5 `guards/internal_terms.go` (`PostGuard`): blocklist fechada de termos internos → sanitiza/override.
- [x] 3.6 `guards/success_without_tool.go` (`PostGuard`): marcador de sucesso sem write-tool bem-sucedido
  e sem verbatim → fallback + Failed.
- [x] 3.7 `mecontrola_agent.go`: trocar `WithMultiItemGuard(...)` por `WithGuardChain(...)`.
- [x] 3.8 Métrica `agent_guard_decisions_total{agent_id, guard, decision}` (decision ∈ {pass, handled}).

## Detalhes de Implementação

Ver `adr-001-guard-chain-cor.md` e `techspec.md` → "Interfaces Chave" e a tabela "Guardas (enforcement)
vs Scorers". Interface `agent.Agent{ID, Instructions, Execute, Stream}` (`ports.go:118`). O
`MultiItemGuard` atual embute `agent.Agent` e sobrescreve só `Execute` (`multi_item_guard.go:55`),
retornando `ToolOutcomeClarify` + `MultiItemOrientationMessage`. Handlers pós-LLM agem só sobre violação
inequívoca (não reescrever resposta válida). Nenhum SQL/regra de domínio no guard (R-AGENT-WF-001.2).
O handler `card_provenance` é entregue em 4.0.

## Critérios de Sucesso

- Cadeia integrada; multi-item equivalente ao guard atual (mensagem verbatim + `clarify`).
- `BuildMeControlaAgent` mantém assinatura pública; `Stream` delegado.
- `PreGuard` curto-circuita sem chamar o LLM (verificável em teste com provider mockado).
- Handlers pós-LLM não alteram respostas válidas (regressão no golden — 6.0).
- Zero comentários em Go de produção (R-ADAPTER-001.1); `go build/vet/test -race` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — cria decorator do agente e handlers no consumidor `internal/agents` do stack mecontrola.
- `design-patterns-mandatory` — materializa o padrão Chain of Responsibility (seletor `status=ok`, primário CoR).
- `domain-modeling-production` — modela `GuardDecision` e a decisão do handler como tipos fechados (state-as-type).

## Testes da Tarefa

- [ ] Testes unitários: cada `PreGuard`/`PostGuard` table-driven (trata/passa); equivalência do
  multi-item; caso `R$ 1.234,56`; ordem e curto-circuito da cadeia; emissão de métrica por handler.
- [ ] Testes de integração: não aplicável (E2E real-LLM em 6.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/guard_chain.go` (novo)
- `internal/agents/application/agents/guards/*.go` (novo)
- `internal/agents/application/agents/multi_item_guard.go` (absorvido/removido)
- `internal/agents/application/agents/mecontrola_agent.go`
