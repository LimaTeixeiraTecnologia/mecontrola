# Registro de Decisão Arquitetural (ADR-007)

## Metadados

- **Título:** Contrato de execução LLM — tool rounds configuráveis, modelo e structured I/O do onboarding
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Time de plataforma; dono do produto
- **Relacionados:** PRD (RF-21.2/RF-38/RF-39, D-22), techspec.md; ADR-002/ADR-004/ADR-006; `internal/platform/agent/agent.go:15`; `.agents/skills/mastra`; `.claude/skills/go-implementation`; memórias `project_agent_haiku_fallback_parse_broken`, `project_onboarding_llm_model`

## Contexto

O agente depende de **tool-calling** (operação diária) e **structured output `Strict:true`** (parse de onboarding e classificação). Três fatos do código/experiência condicionam o design:

1. O loop tem teto fixo `maxToolRounds = 5` (`internal/platform/agent/agent.go:15`); ao exceder retorna `ErrMaxToolRounds`. Múltiplos lançamentos por mensagem (D-22), cada um podendo exigir classificar+registrar, estouram 5 rounds.
2. `json_schema strict` quebra em alguns modelos (haiku/gpt-5-nano por memória) e flash-lite é flaky em tool-calling; o default vigente do projeto é `openai/gpt-4o-mini` (commit recente).
3. O onboarding exige mensagens no tom do produto (RF-12) e extração confiável de valores (renda, objetivo, alocações).

## Decisão

1. **Tool rounds configuráveis**: estender `internal/platform/agent` com `WithMaxToolRounds(n int) AgentOption` (default preservado em 5 para não afetar outros consumidores). O `MeControlaAgent` usa **default 12**. O ledger de idempotência (ADR-004) garante que mais rounds não causem duplicatas; `ErrMaxToolRounds` continua protegendo contra loop infinito.
2. **Modelo `openai/gpt-4o-mini`** (default vigente) para o agente, com **gate `RUN_REAL_LLM`** validando tool-calling + `Strict:true` no chain real antes de produção. Provider único (OpenRouter); sem fallback. Mitigação se inadequado: trocar modelo via config.
3. **Onboarding I/O**: mensagens de cada etapa geradas por step de workflow que chama `agent.Stream` (call-site sancionada, R-AGENT-WF-001.4); respostas do usuário extraídas por `llm.StructuredContract[T]` com `Strict:true` (determinismo pelo schema). Nenhum LLM fora das call-sites sancionadas.

## Alternativas Consideradas

- **Parsear itens 1x + executor determinístico fora do tool-loop** — mais determinístico, mas desvia do fluxo canônico tool-calling do substrato (R-AGENT-WF-001.1) e da skill `mastra`. Rejeitada (mantida como mitigação futura).
- **Limitar itens por mensagem** — pior UX, contraria D-22. Rejeitada.
- **Modelo por classe de tarefa (parse vs conversa)** — mais robusto contra quebras de `Strict`, porém mais complexo de operar no MVP. Rejeitada agora; reavaliar se o gate falhar.
- **gemini/mistral para tudo** — `Strict` comprovado, mas tool-calling menos validado aqui e muda o default recém-adotado. Rejeitada.
- **Mensagens templadas + parse por regex** — determinístico e barato, mas mensagens rígidas (contraria RF-12) e parsing frágil. Rejeitada.

## Consequências

### Benefícios Esperados

- Múltiplos lançamentos sem estourar o teto; fluxo canônico preservado; parse confiável e mensagens no tom do produto.

### Trade-offs e Custos

- Default 12 aumenta custo/latência em mensagens grandes; LLM-judged e Stream consomem tokens (mitigado por amostragem/uso pontual).

### Riscos e Mitigações

- **Modelo inadequado a `Strict`/tool-calling** → gate `RUN_REAL_LLM` obrigatório; troca via config; (futuro) modelo por classe de tarefa.
- **Custo de rounds** → idempotência evita dano em retry; monitorar `agent_tool_invocations_total`.

## Plano de Implementação

1. `WithMaxToolRounds` no primitivo + teste (default 5 inalterado).
2. Config do modelo + gate `RUN_REAL_LLM` no chain real.
3. `StructuredContract` dos inputs de onboarding + step `Stream` para mensagens.

## Monitoramento e Validação

- `agent_tool_invocations_total`, `agent_llm_tokens_total`, `agent_llm_provider_errors_total`. Gate real verde antes de produção.

## Impacto em Documentação e Operação

- Documentar modelo e teto em config/runbook; registrar o gate como pré-requisito de deploy.

## Revisão Futura

- Revisar para "modelo por classe de tarefa" se o gate apontar fragilidade de `Strict`/tool-calling.
