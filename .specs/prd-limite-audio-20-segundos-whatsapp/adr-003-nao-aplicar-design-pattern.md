# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Não aplicar design pattern na seleção de mensagem por motivo
- **Data:** 2026-08-07
- **Status:** Aceita
- **Decisores:** Agente de engenharia, com gate determinístico da skill design-patterns-mandatory
- **Relacionados:** `techspec.md`, ADR-002, `.agents/skills/design-patterns-mandatory/scripts/select_pattern.py`

## Contexto

A ADR-002 introduz uma decisão por motivo dentro de `replyFor`. Pela governança do repositório, toda mudança passa pelo gate de desenho da skill design-patterns-mandatory, que exige a execução do seletor determinístico antes de recomendar ou descartar um pattern. Os sinais canônicos confirmados por evidência foram `prefer_direct_solution`, `single_variant_only` e `low_change_frequency`, com as restrições `minimize_indirection`, `minimize_class_count`, `preserve_public_contract` e `team_needs_low_cognitive_load`.

## Decisão

Não aplicar padrão. O seletor retornou `status: reject` com a alternativa mais simples vencedora: solução direta com um caso adicional na seleção existente. A saída integral do seletor foi registrada nesta decisão: `primary_pattern: null`, `complementary_pattern: null`, `simpler_alternative: "Usar solucao direta, refactor local ou composicao simples"`. Os argumentos do seletor: economia (retorno insuficiente para formalizar pattern), eficiência (menos indireção e custo cognitivo) e robustez (menos tipos e menos acoplamento reduzem a superfície de falha).

## Alternativas Consideradas

1. Strategy por motivo de rejeição. Desvantagem: um único motivo novo não justifica hierarquia de estratégias; aumentaria tipos, wiring e custo cognitivo sem variação de algoritmo real. Rejeitada pelo seletor e pela análise.
2. Chain of Responsibility para mensagens. Desvantagem: não existe cadeia de handlers independentes; a decisão é um único ponto com precedência fixa. Rejeitada.
3. Registry de mensagens por reason. Desvantagem: indireção nova sem segundo consumidor; o mapa já é expresso pela configuração existente. Rejeitada.

## Consequências

### Benefícios Esperados

- Menor contagem de tipos e de indireção; o diff permanece legível em uma única revisão.
- Custo de teste mínimo: asserções diretas de `ReplyText` sem mocks de colaboradores novos.

### Trade-offs e Custos

- Se o número de mensagens dedicadas por motivo crescer no futuro (três ou mais motivos com copy própria), a solução direta deve ser reavaliada; esta ADR registra o gatilho de revisão.

### Riscos e Mitigações

- Risco: overengineering por pressão futura de novos motivos. Mitigação: o gatilho de revisão acima está explícito e o seletor pode ser reexecutado com os novos sinais.

## Plano de Implementação

1. Implementar a solução direta descrita na ADR-002.
2. Critério de conclusão: nenhum tipo, interface ou colaborador novo introduzido para a seleção de mensagem.

## Monitoramento e Validação

- Revisão de código deve confirmar a ausência de abstrações novas neste ponto.
- Gate de governança de adapter fino permanece verde.

## Impacto em Documentação e Operação

Nenhum artefato operacional adicional.

## Revisão Futura

Reexecutar o seletor quando um terceiro motivo de rejeição com mensagem dedicada for demandado.
