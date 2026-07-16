# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Extração LLM única por turno com schema dedicado (intent + meses) e precedência de quantidade
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Jailton (owner), MeControla agents
- **Relacionados:** PRD (RF-01, RF-04, RF-05, RF-06, RF-07, RF-08, RF-14, RF-15); techspec; regra `R-AGENT-WF-001.4` (LLM em call-site sancionada); skill `mastra`.

## Contexto

A resposta do usuário pode conter intenção (aceitar/recusar), quantidade de meses (numérica ou por extenso) ou ser ambígua. O parsing dos demais steps do onboarding usa saída estruturada única (`agent.Execute` com `llm.Schema` estrito), inclusive com precedência explícita no prompt do budget review (`distributionIntentSystemPrompt:789-796`, "values > personalize > accept"). Dois riscos concretos: (1) o `recurrenceSchema` atual é **compartilhado** com o passo `summary_confirm` (`onboarding_workflow.go:1480`, desserializa `yesNoExtract`) — mudá-lo regride o budget review; (2) o teste full-flow WhatsApp fixa a cadeia de chamadas do agente com `.Once()` — adicionar uma segunda chamada LLM no step quebra a contagem.

## Decisão

Fazer **uma** chamada `agent.Execute` por turno no step, com um schema **novo e dedicado** `recurrenceDecisionSchema` e struct `recurrenceExtract{Intent string; HasMonths bool; Months int}`. O `intent` é enum fechado no schema (`negative|positive|unclear`). O `recurrenceSchema`/`yesNoExtract`/`summaryConfirmSystemPrompt` permanecem intocados para `summary_confirm`. O `recurrenceSystemPrompt` é reescrito no estilo do budget review: extrai intenção, sinaliza `hasMonths`/`months`, converte por extenso (um…doze), define `unclear` quando não há intenção reconhecível; a decisão final (prioridade, limites) é da função pura `DecideRecurrence` (ADR-001), não do LLM. Precedência RF-06 materializada na decisão: quantidade válida 1–12 vence intenção; fora de 1–12 → repergunta de inválido; sem quantidade → positiva=12/negativa=none/unclear=ambiguous.

## Alternativas Consideradas

- **Reutilizar `recurrenceSchema` existente**: quebra `summary_confirm`. Rejeitada (regressão direta).
- **Duas chamadas LLM (intenção; depois meses)**, espelhando `classifyDistributionIntent`+`extractAllocationValues`: mais chamadas, custo e latência maiores, e quebra a contagem `.Once()` do teste full-flow. Rejeitada por custo/eficiência e risco de regressão de teste; uma extração combinada é suficiente aqui (diferente do budget review, que tem 3 intenções e valores por 5 categorias).
- **Decidir a prioridade no prompt**: não determinístico/testável. Rejeitada (a prioridade vive na função pura).

## Consequências

### Benefícios Esperados

- Uma chamada LLM por turno: menor custo/latência, preserva a contagem de chamadas dos testes full-flow (só o payload da fixture muda).
- `summary_confirm` intacto (0 regressão).
- Conversão por extenso e limites tratados de forma consistente com os demais steps.

### Trade-offs e Custos

- Um schema/prompt/struct novos (baixo custo). O prompt precisa ser claro sobre `unclear` e sobre extrair `months` mesmo quando a frase é natural.

### Riscos e Mitigações

- Risco: o LLM classificar ambíguo como negativo (falso "não"). Mitigação: `unclear` explícito no enum + gate real-LLM com cenários ambíguos e asserção de repergunta (ADR-004).
- Risco: extrair `months` fora de 1–12 com intenção positiva. Mitigação: `DecideRecurrence` resolve para `Invalid` (repergunta), nunca aplica (0 falso-sucesso).

## Plano de Implementação

1. Criar `recurrenceDecisionSchema` + `recurrenceExtract`.
2. Reescrever `recurrenceSystemPrompt` (intenção + meses + por extenso + `unclear`).
3. Manter `recurrenceSchema`/`yesNoExtract`/`summary_confirm` inalterados.
4. Atualizar a fixture de recorrência do teste full-flow para o novo schema.

## Monitoramento e Validação

- Sucesso: gate real-LLM ≥0,90 nos 5 tipos; `summary_confirm` continua verde; teste full-flow verde com 1 chamada no step.
- Reverter: se a extração combinada degradar a acurácia abaixo do gate, considerar separar intenção e meses em duas chamadas.

## Impacto em Documentação e Operação

- Techspec; `TestM02_NoRendaTermInAnyOnboardingSurface` passa a incluir os novos prompts.

## Revisão Futura

- Revisitar se surgirem novas formas de resposta não cobertas pelos exemplos do prompt (monitorar via counter `outcome=ambiguous_reprompt`).
