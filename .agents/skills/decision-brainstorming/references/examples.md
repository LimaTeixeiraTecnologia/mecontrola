# Exemplos de Uso

## Exemplo 1 - Solução Prematura
Prompt:
`Use a skill decision-brainstorming para avaliar se devemos migrar o checkout para microserviços.`

Comportamento esperado:
- Tratar microserviços como hipótese.
- Comparar monólito modular, monólito distribuível, microserviços e arquitetura híbrida.
- Registrar decisão preliminar e recomendar `technical-discovery-production` se a direção exigir validação técnica.

## Exemplo 2 - Decisão Estratégica de Produto
Prompt:
`Use a skill decision-brainstorming para decidir entre criar onboarding self-service ou reforçar atendimento humano no primeiro release.`

Comportamento esperado:
- Comparar alternativas com custo, risco operacional, impacto no usuário e capacidade do time.
- Registrar trade-offs aceitos.
- Recomendar `epic-story-discovery` se a decisão já estiver pronta para decomposição de produto.

## Exemplo 3 - Origem em Tracker
Prompt:
`Use a skill decision-brainstorming para analisar a direção preliminar da US PROJ-231 antes de gerar PRD.`

Comportamento esperado:
- Registrar hipóteses e alternativas antes de consolidar PRD.
- Recomendar `tracker-to-prd` como próximo passo se a origem de tracking for necessária para preservar contexto.
