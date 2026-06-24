# ADR-003 — Structured Output com `Strict=true` em Todas as Classes Estruturadas

## Metadados

- **Título:** Contrato LLM↔domínio via JSON Schema estrito (provider `Strict=true`); modelos elegíveis por guard real-LLM
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Solicitante + plataforma
- **Relacionados:** PRD (RF-07, RF-08, RF-18, RF-19), techspec §"Structured Output Strict=true", ADR-002, memórias `project_agent_haiku_fallback_parse_broken`, `project_onboarding_llm_model`

## Contexto

Hoje o parse usa `Strict=false` (`parse_inbound.go:97`) porque, com `Strict=true`, modelos como
haiku/gpt-5-nano quebram o `json_schema` (memória confirmada: só gemini+mistral funcionavam com
`strict=false`). O PRD/solicitante decidiu **exigir `Strict=true`** para tornar o contrato
LLM↔portas-de-módulo à prova de alucinação (RF-07/08): saída inválida nunca vira ação de domínio.
Decisão do solicitante: `Strict=true` aplica-se a **todas as classes estruturadas** (parse e
onboarding); a resposta conversacional (`KindUnknown`) é texto livre por natureza e não é alvo de
schema.

## Decisão

1. `parse_inbound.go` passa `Strict: true`. O `ParseIntentJSONSchema` é ajustado para satisfazer o
   modo estrito do OpenRouter: `additionalProperties:false` (já presente) e **todas as propriedades
   em `required`** (hoje só `kind,confidence`), com defaults vazios/zero que os smart constructors do
   `Intent` já toleram.
2. **Onboarding migra de tool-calling → json_schema response_format com `Strict=true`.** Hoje o
   onboarding usa function/tool-calling (`run_onboarding_turn.go:387-389`: `Tools`/`ToolChoice:"auto"`;
   catálogo em `onboarding_tool_catalog.go`). Decisão: substituir o mecanismo por **Structured Output
   json_schema estrito**, uniforme com o parse — o LLM passa a devolver um objeto tipado (qual passo
   do onboarding + slots), validado por schema estrito, e o `onboarding_tool_dispatcher` passa a
   despachar a partir do objeto estruturado (não de `ToolCalls`). Elimina a heterogeneidade
   tool-calling vs json_schema e torna o contrato único.
3. **Modelos elegíveis** para classes estruturadas limitam-se aos que suportam structured outputs
   estritos de forma confiável, **comprovado por guard de teste com LLM real** (`RUN_REAL_LLM`) antes
   de configurar como primário/fallback. Haiku/gpt-5-nano ficam **inelegíveis** enquanto quebrarem.
4. **Onboarding deixa de usar haiku** por padrão: o modelo de onboarding será revalidado e, se não
   passar no guard, substituído por um elegível.

## Alternativas Consideradas

- **Manter `Strict=false` + validação app-side**: model-agnostic, mas o contrato no provider é mais
  fraco e o solicitante pediu explicitamente `Strict=true`. Rejeitada por decisão de produto.
- **`Strict=true` só no parse**: preservaria haiku no onboarding; rejeitada — solicitante escolheu
  "todas as classes" para consistência do contrato.

## Consequências

### Benefícios Esperados

- Contrato tipado garantido pelo provider; menos alucinação/saída malformada; mapeamento 1:1 para as
  portas mais seguro.

### Trade-offs e Custos

- Reduz o conjunto de modelos elegíveis; exige revalidar onboarding (custo de tuning) e manter o guard
  real-LLM como gate.

### Riscos e Mitigações

- **Risco:** modelo elegível para onboarding com qualidade inferior a haiku. **Mitigação:** guard
  real-LLM compara qualidade antes do cutover; fallback chain por classe. **Rollback:** reverter
  `Strict=false` no parse e reapontar onboarding para haiku (documentado).
- **Risco:** `required` completo no schema rejeitar saídas válidas. **Mitigação:** defaults vazios/zero
  + smart constructors tolerantes; cobertura por guard real-LLM e testes de decode.
- **Risco (alto):** migrar onboarding de tool-calling para json_schema reescreve um fluxo que já
  funciona (`run_onboarding_turn`, `onboarding_tool_catalog`, `onboarding_tool_dispatcher`).
  **Mitigação:** preservar a semântica de passos do onboarding (mesmos slots/etapas do Documento
  Oficial), cobrir com testes e guard real-LLM antes do cutover; manter feature parity verificável
  (mesmas respostas/etapas). **Rollback:** reverter o interpreter de onboarding para tool-calling.

## Plano de Implementação

1. Ajustar schema (`required` completo). 2. `Strict:true` no parse e onboarding. 3. Guard real-LLM por
   classe (parte do ADR-002). 4. Selecionar/validar modelo de onboarding elegível. 5. Atualizar
   `TestParseInbound_RealLLM_ProductionChain` para Strict=true.

## Monitoramento e Validação

- `agent_intent_parsed_total{kind,outcome}` sem aumento de `fallback_invalid_json`; guard real-LLM
  verde por modelo/classe. Sucesso: 100% das ações nascem de Structured Output validado.

## Impacto em Documentação e Operação

- Atualizar memórias `project_onboarding_llm_model` e `project_agent_haiku_fallback_parse_broken`,
  runbook de parser/policy, `.env.example`.

## Revisão Futura

- Revisar elegibilidade quando novos modelos OpenRouter suportarem strict de forma confiável.
