# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Não aplicar design pattern GoF na emissão do typing indicator
- **Data:** 2026-08-07
- **Status:** Aceita
- **Decisores:** agente de especificação, com seleção determinística da skill design-patterns-mandatory
- **Relacionados:** `.specs/prd-indicador-digitando-whatsapp/techspec.md`, `adr-001`, `adr-002`

## Contexto

A skill design-patterns-mandatory exige a execução do seletor determinístico `scripts/select_pattern.py` antes de recomendar qualquer padrão. A mudança consiste em uma chamada HTTP adicional na cadeia de envio existente, controlada por flag, sem variação de algoritmo, sem estado novo e sem composição estrutural nova.

Entrada do seletor: sinais `prefer_direct_solution`, `low_change_frequency`, `single_variant_only`, `remote_boundary`, `strict_test_isolation`; restrições `minimize_class_count`, `minimize_indirection`, `preserve_public_contract`, `avoid_inheritance`, `team_needs_low_cognitive_load`.

Saída do seletor: `status: reject`, com a alternativa mais simples "usar solução direta, refactor local ou composição simples" e os argumentos de economia, eficiência e robustez registrados na saída do script.

## Decisão

Não aplicar nenhum padrão GoF. A solução é direta: um método novo em cada elo da cadeia existente (`meta.Client`, `WhatsAppGateway`, interface do consumidor) e uma option no padrão de functional options que o consumer já usa (`ConsumerOption`, consumer :87-119), que é convenção preexistente do arquivo e não introdução de pattern novo.

## Alternativas Consideradas

- Decorator sobre o sender para adicionar o typing como comportamento transversal.
  - Vantagens: emissão transparente ao consumer.
  - Desvantagens: esconde a chamada de rede em uma camada de indireção; dificulta o controle fino do ponto de emissão (após dedup, antes do processamento); aumenta a superfície de teste.
  - Motivo da rejeição: indireção sem variação real de comportamento; o seletor não pontuou nenhum pattern candidato.
- Strategy para alternar emissores de presença.
  - Vantagens: preparado para múltiplos canais.
  - Desvantagens: existe um único canal e uma única variante; `single_variant_only` é bloqueador explícito.
  - Motivo da rejeição: generalização especulativa proibida pelo modo de trabalho do AGENTS.md.

## Consequências

### Benefícios Esperados

- Menor número de tipos e de indireções possível; leitura linear do fluxo.
- Custo de teste mínimo e proporcional ao risco.

### Trade-offs e Custos

- Se no futuro houver múltiplos canais de presença, a extração de uma abstração será decidida com evidência nova e novo registro.

### Riscos e Mitigações

- Risco: falso positivo futuro de "deveria ter usado pattern". Mitigação: esta ADR registra a saída do seletor e os sinais usados, permitindo repetir a decisão deterministicamente.

## Plano de Implementação

1. Implementar a solução direta descrita na techspec.
2. Critério de conclusão: nenhum tipo novo além dos payloads de request e das assinaturas de método descritos na techspec.

## Monitoramento e Validação

- Revisão de código deve confirmar a ausência de abstrações além das descritas; qualquer pattern introduzido depois exige nova execução do seletor.

## Impacto em Documentação e Operação

- Nenhum.

## Revisão Futura

- Revisitar apenas se surgir segundo canal de presença ou variação real de algoritmo de emissão.
