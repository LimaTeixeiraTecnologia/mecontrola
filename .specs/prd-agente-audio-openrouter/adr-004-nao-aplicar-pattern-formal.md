# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** Nao aplicar pattern formal novo
- **Data:** 2026-08-04
- **Status:** Aceita
- **Decisores:** Engenharia Me Controla
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD proibe novo agente, workflow kernel especifico de audio, fallback chain e regra financeira
duplicada. O codebase ja possui adapters finos, DI manual, `internal/platform/llm`,
`internal/platform/whatsapp` e consumidor agentivo `internal/agents`.

O seletor deterministico de `design-patterns-mandatory` foi executado com sinais reais:
`external_interface_mismatch`, `remote_boundary`, `prefer_direct_solution`, `single_variant_only` e
`low_change_frequency`. Resultado: `status=reject`.

## Decisao

Nao aplicar pattern formal novo. A implementacao deve usar solucao direta: tipos fechados, use case de
orquestracao, clients/adapters finos e DI manual existente.

## Alternativas Consideradas

- Adapter formal novo como pattern primario: rejeitado porque adapters finos ja sao convencao local e nao
  exigem nova estrutura decisoria.
- Facade: rejeitado porque criaria camada de orquestracao opaca sem ganho sobre use case explicito.
- Strategy: rejeitado porque ha um unico provider e nenhuma troca runtime permitida.
- Proxy/Decorator: rejeitados porque nao ha cache, lazy loading ou empilhamento de responsabilidades.

## Consequencias

### Beneficios Esperados

- Menor numero de tipos.
- Menor custo cognitivo.
- Mais facil auditar o ponto exato onde audio pode ou nao virar texto canonico.

### Trade-offs e Custos

- Se futuros canais/modalidades surgirem, a decisao devera ser reavaliada com novo input do seletor.

### Riscos e Mitigacoes

- Risco: orquestracao crescer demais no use case.
- Mitigacao: decompor em metodos pequenos `validate`, `download`, `transcribe`, `decide`, `persist`,
  `dispatch`, sem criar pattern ate haver variacao real.

## Plano de Implementacao

1. Criar tipos e interfaces pequenas.
2. Implementar use case direto.
3. Manter clients/adapters finos.
4. Revisar novamente se surgir segunda modalidade ou segundo provider.

## Monitoramento e Validacao

- Review deve confirmar ausencia de novo agente, fallback chain e workflow kernel de audio.
- Testes devem provar bloqueio de dispatch em outcome nao aprovado.

## Impacto em Documentacao e Operacao

Registrar a decisao na techspec e tasks para evitar overengineering futuro.

## Revisao Futura

Revisar quando houver mais de um canal de audio, mais de um provider permitido ou multiplas politicas de
transcricao em runtime.
