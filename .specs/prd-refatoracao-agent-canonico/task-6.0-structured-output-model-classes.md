# Tarefa 6.0: Structured Output Strict=true + roteamento por classe + onboarding json_schema

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Formalizar o contrato LLM↔domínio: `Strict=true` no parse (schema `required` completo), roteamento de
modelo por classe de tarefa (parse/onboarding/conversacional) com fallback+breaker por classe, e
migração do onboarding de tool-calling para json_schema. Preserva a política de confiança e o pending
step de clarificação.

<requirements>
- RF-07: Structured Output tipado com provider Strict=true no parse; aplica-se às classes estruturadas (parse + onboarding); conversacional KindUnknown é texto livre.
- RF-08: saída estruturada inválida não vira ação de domínio (clarificação/fallback determinístico).
- RF-11: faltando info, perguntar só o que falta, uma pergunta por vez, com pending step durável.
- RF-13: intenções/slots/outcomes/estados de espera são tipos fechados (state-as-type).
- RF-14: modelo configurável por classe de tarefa (primário + fallbacks).
- RF-15: fallback chain + circuit breaker independentes por classe.
- RF-16: seleção por classe observável (métrica class/model/outcome), cardinalidade controlada.
- RF-17: manter política de confiança (writes abaixo do limiar bloqueados/clarificados).
- RF-18: modelos elegíveis limitados aos que suportam strict, comprovado por guard real-LLM.
- RF-19: modelo de onboarding revalidado sob Strict=true (deixa haiku se não passar).
- RF-25: onboarding conduz as etapas oficiais (preservar passos/slots na migração para json_schema).
</requirements>

## Subtarefas

- [ ] 6.1 `LLMClass` (tipo fechado) + `ClassRouter` + `AgentModelClassConfig{Primary,Fallbacks,MaxTokens}` por classe; `module.go` constrói 3 chains via `buildLLMChain`.
- [ ] 6.2 `parse_inbound.go`: `Strict:true`; ajustar `ParseIntentJSONSchema` para `required` completo + `additionalProperties:false`, com defaults vazios/zero tolerados pelos smart constructors.
- [ ] 6.3 Migrar onboarding tool-calling → json_schema: `run_onboarding_turn.go` deixa `Tools`/`ToolChoice`; objeto estruturado tipado; `onboarding_tool_dispatcher` despacha do objeto (não de `ToolCalls`); preservar etapas do Documento Oficial.
- [ ] 6.4 Migrar envs para por-classe (`AGENT_LLM_PARSE_*`, `AGENT_LLM_ONBOARDING_*`, `AGENT_LLM_CONVERSATIONAL_*`); `.env.example`; manter compat de defaults.
- [ ] 6.5 Métrica `agent_llm_class_total{class,model,outcome}` (cardinalidade controlada); preservar política de confiança (RF-17) e pending step de clarificação (RF-11).
- [ ] 6.6 Guard real-LLM (`RUN_REAL_LLM`) por classe valida Strict=true antes de promover primário; selecionar modelo de onboarding elegível.

## Detalhes de Implementação

Ver `adr-002-model-routing-by-class.md`, `adr-003-structured-output-strict.md` e techspec §"Roteamento
de modelo por classe", §"Structured Output Strict=true". DMMF: `LLMClass` discriminated/tipo fechado.

## Critérios de Sucesso

- Parse com Strict=true e schema required-completo; `agent_intent_parsed_total` sem alta de invalid_json.
- 3 classes com fallback/breaker independentes; métrica por classe.
- Onboarding em json_schema com paridade de etapas (guard real-LLM verde); onboarding sem haiku se reprovado.
- Política de confiança e clarificação preservadas.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — altera a fronteira LLM/parse, system prompt e onboarding do `internal/agent` (R-AGENT-WF-001.4/.8, parse boundary).

## Testes da Tarefa

- [ ] Testes unitários (ClassRouter; ParseInbound Strict=true decode + required; confidence clamp; onboarding dispatch de objeto estruturado).
- [ ] Testes de integração (guard real-LLM por classe sob `RUN_REAL_LLM`; paridade de etapas do onboarding).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/usecases/parse_inbound.go`, `application/prompting/prompts.go`
- `internal/agent/application/usecases/run_onboarding_turn.go`, `onboarding_tool_catalog.go`
- `internal/agent/infrastructure/onboarding/onboarding_tool_dispatcher.go`
- `internal/agent/module.go`, `configs/config.go`, `.env.example`
- `internal/agent/application/interfaces/llm_provider.go`, `infrastructure/providers/openrouter/client.go`
