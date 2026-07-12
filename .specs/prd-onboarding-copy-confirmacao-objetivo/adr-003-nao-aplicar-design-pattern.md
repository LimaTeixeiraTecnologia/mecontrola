# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Não aplicar design pattern (gate design-patterns-mandatory)
- **Data:** 2026-07-12
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD e techspec `.specs/prd-onboarding-copy-confirmacao-objetivo/`; skill `.agents/skills/design-patterns-mandatory`; ADR-001, ADR-002

## Contexto

A skill `design-patterns-mandatory` exige um gate explícito "aplicar" vs. "não aplicar padrão" para toda mudança de desenho. Esta funcionalidade é composta por: (1) troca de fragmentos em constantes de string; (2) uma função pura de formatação (`goalConfirmationReprompt`); (3) remoção do emoji 💳 em mensagens; (4) remoção do objetivo na frase de conclusão (com simplificação de assinatura de `conclusionFinalMessage`).

## Decisão

**Não aplicar** nenhum design pattern GoF. As mudanças não introduzem nova abstração, ponto de extensão, polimorfismo, colaboração de objetos, variação de comportamento em runtime ou família de tipos que justifique Factory, Strategy, Template Method, Decorator, State, ou qualquer outro padrão. A montagem de mensagem permanece por constantes e `fmt.Sprintf`, e a única unidade nova é uma função pura de uma responsabilidade.

## Alternativas Consideradas

- **Strategy para variar a copy de reforço**: introduziria uma interface e implementações para uma única variante determinística — complexidade sem benefício (YAGNI). Rejeitado.
- **Template Method para montagem de mensagens de cartão**: as mensagens são strings estáticas/`Sprintf` sem esqueleto de algoritmo compartilhado que justifique herança/hook. Rejeitado.
- **Builder para o "Resumo de Onboarding"**: já existe `strings.Builder` idiomático; um Builder de domínio seria over-engineering. Rejeitado.

## Consequências

### Benefícios Esperados

- Menor complexidade e menor superfície de manutenção; código idiomático Go.
- Aderência a `go-implementation` (preferir tipos concretos; evitar abstração especulativa) e a object-calisthenics apenas onde há invariante real.

### Trade-offs e Custos

- Se, no futuro, múltiplas variantes de reforço/estilo de copy forem exigidas (ex.: A/B, personalização), poderá ser necessário revisitar e introduzir Strategy — registrado como gatilho de revisão.

### Riscos e Mitigações

- **Risco:** crescimento futuro da variação de copy sem estrutura. **Mitigação:** este ADR documenta o gatilho para reavaliar (ver Revisão Futura).

## Plano de Implementação

1. Implementar as mudanças como copy + função pura, sem novas interfaces/tipos.
2. Revisão de diff confirma ausência de abstração especulativa.

## Monitoramento e Validação

- Critério de sucesso: diff contém apenas constantes, uma função pura e ajustes de assinatura; nenhuma nova interface/tipo de domínio.

## Impacto em Documentação e Operação

- Apenas PRD/techspec desta pasta.

## Revisão Futura

- Reavaliar se surgir necessidade de múltiplas variantes de copy (personalização, A/B, i18n), quando Strategy/Template Method poderá passar a se justificar.
