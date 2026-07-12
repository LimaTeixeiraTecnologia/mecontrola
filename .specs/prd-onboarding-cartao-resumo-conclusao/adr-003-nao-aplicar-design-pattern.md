# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Não aplicar design pattern — copy localizada + função pura de formatação
- **Data:** 2026-07-12
- **Status:** Aceita
- **Decisores:** Time de plataforma (agents)
- **Relacionados:** `.specs/prd-onboarding-cartao-resumo-conclusao/techspec.md`, skill `design-patterns-mandatory`

## Contexto

A skill `design-patterns-mandatory` exige rodar o seletor determinístico antes de recomendar (ou dispensar) qualquer padrão. A mudança consiste em: (1) reescrever três funções de prompt de cartão para conter "cartão 💳" e um exemplo; (2) destacar a palavra "outro" com `**`; (3) adicionar uma função pura `conclusionSummaryMessage` que formata uma string a partir de estado já existente e de duas leituras de fonte de verdade. Não há criação de família de objetos, variação de algoritmo em runtime, travessia de estrutura, coordenação entre objetos nem acoplamento estrutural a desacoplar.

O seletor (`scripts/select_pattern.py`) retornou `needs_more_evidence` — nenhum sinal estrutural canônico presente —, o que, combinado com os gates de economia/eficiência da skill, indica que a solução direta vence.

## Decisão

Não aplicar nenhum design pattern. Implementar como código direto: constantes/funções de prompt e uma função pura de montagem de mensagem, reutilizando helpers existentes (`renderAllocationLines`, `categoryLabels`, `canonicalSlugs`, `money.FromCents`). A única mudança estrutural é ampliar a assinatura de `BuildConclusionStep` (injeção de dependências já existentes), o que é wiring padrão do projeto, não um pattern.

## Alternativas Consideradas

- **Builder para a mensagem de resumo:** Vantagem: encapsular a montagem. Desvantagem: uma única forma de resumo, sem variação combinatória; Builder adicionaria tipo e indireção sem ganho. Rejeitada (overengineering).
- **Strategy para variações de copy (com/sem cartão, com/sem meta):** Desvantagem: as variações são simples ramos condicionais em função pura; Strategy multiplicaria tipos para um branching trivial. Rejeitada.
- **Template Method para o passo de conclusão:** Desvantagem: não há hierarquia de passos nem esqueleto compartilhado a parametrizar. Rejeitada.

## Consequências

### Benefícios Esperados

- Menor custo cognitivo e de manutenção; nenhuma indireção nova.
- Aderência a R-ADAPTER-001.1 (zero comentários) e à preferência do projeto por tipos concretos e mudança mínima.

### Trade-offs e Custos

- A função de montagem cresce com ramos condicionais simples; aceitável e testável por exact-copy.

### Riscos e Mitigações

- Risco: crescimento futuro da copy justificar refatoração. Mitigação: se surgirem múltiplas variações reais de resumo, reabrir a decisão com novo input ao seletor.

## Plano de Implementação

1. Implementar as funções de prompt e `conclusionSummaryMessage` diretamente.
2. Não introduzir tipos/abstrações novas.

## Monitoramento e Validação

- Revisão de código confirma ausência de indireção desnecessária e conformidade com zero comentários.

## Impacto em Documentação e Operação

- Nenhum além desta ADR como registro do verdict do seletor.

## Revisão Futura

- Revisitar apenas se a montagem do resumo passar a ter múltiplas variações estruturais (novos canais, formatos condicionais ricos) que caracterizem sinal canônico de pattern.
</content>
