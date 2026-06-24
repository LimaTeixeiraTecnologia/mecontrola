# ADR-002 — Roteamento de Modelo por Classe de Tarefa

## Metadados

- **Título:** Seleção de modelo OpenRouter por classe (parse / onboarding / conversacional)
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Plataforma / dono do `internal/agent`
- **Relacionados:** PRD (RF-14, RF-15, RF-16), techspec §"Roteamento de modelo por classe", ADR-003

## Contexto

Hoje há **modelo único + fallback** global (`AGENT_LLM_PRIMARY_MODEL=google/gemini-2.5-flash-lite`,
`AGENT_LLM_FALLBACK_MODELS=mistralai/...`) e um modelo separado de onboarding
(`AGENT_ONBOARDING_LLM_MODEL=anthropic/claude-haiku-4.5`). O PRD pede usar "os modelos com bom score
dado as intenções", decidido como **roteamento por classe de tarefa**: parse de intenção (rápido/
barato/estruturado), onboarding (conversa guiada estruturada) e resposta conversacional/fallback
(texto livre). Cada classe tem requisitos distintos de custo, latência e qualidade.

## Decisão

Introduzir `LLMClass` (tipo fechado: `LLMClassParse`, `LLMClassOnboarding`, `LLMClassConversational`)
e um `ClassRouter` que resolve, por classe, uma `FallbackChain` própria (primário + fallbacks) com
`CircuitBreaker` independente. Config por classe (`AgentModelClassConfig{Primary, Fallbacks,
MaxTokens}`) substitui os campos planos atuais; as envs existentes mapeiam para a classe equivalente
e novas envs por classe são adicionadas. `buildLLMChain` (já existente) é reusado, agora invocado por
classe. `ParseInbound` recebe `LLMClassParse`; onboarding `LLMClassOnboarding`; conversacional
`LLMClassConversational`.

## Alternativas Consideradas

- **Por intenção individual** (cada `intent.Kind` → modelo): máximo controle, mas explode config e
  tuning, e o parse é um único call-site (a intenção só é conhecida após o parse). Rejeitada —
  complexidade desproporcional ao ganho no MVP.
- **Modelo único + fallback** (status quo): simples, mas não aproveita modelos melhores por tipo de
  tarefa. Rejeitada pelo objetivo de eficiência.

## Consequências

### Benefícios Esperados

- Melhor relação custo/qualidade/latência por classe; fallback/breaker isolados evitam contágio.

### Trade-offs e Custos

- Mais entradas de config e mais combinações a validar (guard real-LLM por classe — ADR-003).

### Riscos e Mitigações

- **Risco:** divergência de comportamento entre classes. **Mitigação:** contrato de saída comum
  (Structured Output Strict=true nas classes estruturadas); conversacional permanece texto livre.
- **Rollback:** apontar as três classes para o mesmo modelo (equivale ao comportamento atual).

## Plano de Implementação

1. `LLMClass` + `ClassRouter` + `AgentModelClassConfig`. 2. `module.go` constrói 3 chains.
3. Injetar interpreter por classe em parse/onboarding/conversacional. 4. Migrar envs + `.env.example`.
5. Guard real-LLM por classe (ADR-003).

## Monitoramento e Validação

- Métrica `agent_llm_class_total{class,model,outcome}` (cardinalidade controlada). Sucesso: cada
  classe usa seu modelo; fallback/breaker acionam isoladamente.

## Impacto em Documentação e Operação

- `.env.example`, runbook de parser/policy, dashboard de LLM.

## Revisão Futura

- Reavaliar granularidade (por intenção) se surgir caso de uso com ganho comprovado.
