# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Confirmação + reforço do objetivo determinístico (função pura, sem LLM)
- **Data:** 2026-07-12
- **Status:** Aceita
- **Decisores:** Solicitante do produto (múltipla escolha 2026-07-12), time de plataforma
- **Relacionados:** PRD `.specs/prd-onboarding-copy-confirmacao-objetivo/prd.md` (RF-03, RF-04, RF-05, RF-06); techspec da mesma pasta; DMMF `.agents/skills/domain-modeling-production`; R-AGENT-WF-001.4 (LLM só nas call-sites sancionadas)

## Contexto

A segunda mensagem do onboarding (após o usuário informar o objetivo) hoje vai direto à pergunta opcional do valor da meta (`goalValueReprompt`), sem confirmar nem reforçar o objetivo declarado. O solicitante quer que essa mensagem confirme o objetivo (ecoando o texto do usuário) e traga um reforço positivo. Há duas formas de gerar o reforço: (a) determinística por template, ou (b) personalizada por LLM. Restrições: onboarding é fluxo durável determinístico; adicionar LLM aumenta custo, latência e não-determinismo, e cria nova call-site (R-AGENT-WF-001.4 restringe LLM às call-sites sancionadas).

## Decisão

Gerar a confirmação+reforço de forma **determinística**, por uma função **pura** `goalConfirmationReprompt(goal string) string` que interpola o objetivo do usuário e prefixa a pergunta opcional de valor já existente:

```
Perfeito! Anotei seu objetivo: "<goal>" 🎯 Vamos juntos tornar isso realidade! 💪

<goalValueReprompt>
```

A função é chamada nos dois pontos de emissão da pergunta de valor (`BuildGoalStep`, linhas 768 e 775). Nenhuma call-site de LLM é adicionada. O exemplo de formato de valor em `goalValueReprompt` é alinhado a "R$ 5.000,00"/"5 mil" para coerência com as boas-vindas.

## Alternativas Consideradas

- **Reforço personalizado via LLM**: mais natural, porém adiciona custo/latência, não-determinismo e uma nova call-site de LLM no fluxo durável; rejeitado pelo solicitante e por conflito com a economia/robustez do onboarding.
- **Mensagem separada (dois outbounds)**: confirmação numa mensagem e pergunta de valor noutra; rejeitado — o solicitante quer uma única mensagem prefixando a confirmação à pergunta já existente.
- **Remover a pergunta de valor e só confirmar**: rejeitado — a coleta opcional de valor deve ser preservada (RF-05).

## Consequências

### Benefícios Esperados

- Determinismo total: testável por unit test puro, sem mock de LLM.
- Zero custo/latência adicional; preserva a economia do onboarding.
- Aderência a DMMF (função pura, sem IO) e a R-AGENT-WF-001.4 (nenhuma nova call-site de LLM).

### Trade-offs e Custos

- O reforço é fixo (não adaptado semanticamente ao tipo de objetivo). Aceito pelo solicitante — o eco do objetivo já entrega a sensação de "fui ouvido".

### Riscos e Mitigações

- **Risco:** objetivo com caracteres inesperados no eco. **Mitigação:** é texto do próprio usuário renderizado como conteúdo de mensagem WhatsApp (sem execução); sem risco de injeção. **Rollback:** reverter a chamada para `goalValueReprompt` puro.

## Plano de Implementação

1. Adicionar `goalConfirmationReprompt` (pura) e alinhar o exemplo de valor em `goalValueReprompt`.
2. Substituir os dois `suspendStep(state, goalValueReprompt)` por `suspendStep(state, goalConfirmationReprompt(state.Goal))`.
3. Adicionar unit test puro para a função; atualizar asserts 817/858/910.

## Monitoramento e Validação

- Critério de sucesso: a 2ª mensagem contém o objetivo ecoado entre aspas + reforço + a pergunta de valor; nenhuma nova chamada de LLM medida no fluxo.
- Sinais: unit tests verdes; gate golden agregado inalterado.

## Impacto em Documentação e Operação

- PRD e techspec desta pasta. Sem runbook/alerta afetado.

## Revisão Futura

- Revisar se o produto decidir personalizar o reforço por objetivo (exigiria reavaliar custo/latência/call-site de LLM).
