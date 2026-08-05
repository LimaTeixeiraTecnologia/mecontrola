# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Não aplicar pattern formal no contrato inicial
- **Data:** 2026-08-05
- **Status:** Aceita
- **Decisores:** Engenharia MeControla
- **Relacionados:** `.specs/prd-alertas-proativos/techspec.md`

## Contexto

A primeira fatia precisa evoluir contrato compartilhado e adapter WhatsApp já existentes. O client Meta já possui `SendTemplate`, e o problema não exige família de produtos, runtime strategy, facade de subsistema complexo ou decorator.

## Decisão

Não aplicar design pattern formal. A solução deve usar evolução direta de tipos, métodos e adapters finos.

## Alternativas Consideradas

- Strategy: rejeitada porque não há variação de algoritmo em runtime.
- Facade: rejeitada porque o subsistema não exige orquestração complexa nova.
- Decorator: rejeitada porque não há responsabilidade opcional empilhável.
- Abstract Factory: rejeitada porque não há família de produtos.

## Consequências

### Benefícios Esperados

- Menor indireção e menor custo cognitivo.
- Menor blast radius nos módulos existentes.
- Testes mais diretos e objetivos.

### Trade-offs e Custos

- Se múltiplos canais ou variações complexas surgirem no futuro, a decisão poderá ser revista.

### Riscos e Mitigações

- Risco: contrato crescer demais. Mitigação: manter `TemplateMessage` pequeno e validar por uso real.

## Plano de Implementação

1. Evoluir contrato compartilhado de notification.
2. Manter adapters finos.
3. Cobrir regressões com testes locais e de fronteira.

## Monitoramento e Validação

- Build e testes dos módulos que implementam `ChannelGateway`.
- Revisão de diff para confirmar ausência de branching de negócio no adapter.

## Impacto em Documentação e Operação

Registrar esta ADR como justificativa para não introduzir pattern formal.

## Revisão Futura

Revisar se houver mais de um canal real com comportamentos divergentes ou variação operacional comprovada.
