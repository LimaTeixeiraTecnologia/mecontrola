# Testes

<!-- TL;DR
Diretrizes de testes em Go com severidade proporcional: testes devem cobrir comportamento observavel; suite, mockery e integracao entram quando a superficie realmente exigir.
Keywords: teste, suite, mockery, integração, cobertura
Load complete when: tarefa envolve estratégia de testes, revisão de testes unitários ou de integração, ou decisão de gates de validação.
-->

## Objetivo
Garantir correção e prevenir regressão com custo proporcional ao risco, reduzindo falso positivo.

## Estratégia
- Começar por teste local no pacote alterado.
- Promover para integração apenas quando a fronteira real justificar.
- Preservar o padrão já dominante do pacote afetado, em vez de impor uma forma única ao repositório inteiro.

## Severidade por contexto

### R3 — `mockery.yml` `[HARD contextual]`
Aplicar como bloqueante apenas quando o diff:
- alterar interface coberta por mocks gerados; ou
- introduzir novo mock gerado como parte da estratégia de teste.

Nesses casos:
- `mockery.yml` deve existir no módulo relevante;
- `with-expecter: true` continua obrigatório;
- `mockery --config mockery.yml --dry-run` passa a fazer parte dos gates do escopo alterado.

Quando nao houver mock gerado no diff, a ausencia de `mockery` nao e falha por si so.

### R4 — `testify/suite` `[HARD contextual]`
Aplicar como bloqueante apenas quando:
- o pacote alterado já usa `suite.Suite` como padrão consolidado; ou
- o novo teste exigir setup/teardown compartilhado, reset de mocks ou múltiplos cenários stateful.

Quando a mudança for simples e puramente funcional:
- `require`/`assert` com teste direto pode ser suficiente;
- table-driven tests continuam preferidos, mas sem obrigatoriedade universal de `suite`.

## Unit Tests
- Cobrir comportamento de dominio, use case e lógica pura.
- Nomear cenários pelo comportamento.
- Evitar dependência externa real, `time.Sleep` e estado global.
- Preferir mocks/fakes pequenos quando realmente isolarem a fronteira com menor custo.
- Ler `testing-unit.md` para decisões de forma e severidade.

## Integration Tests
- Acionar apenas quando a fronteira de I/O alterada não puder ser coberta confiavelmente por unit test.
- Preferir `testcontainers-go` ou infraestrutura efêmera equivalente.
- Separar por build tag quando o custo operacional justificar.
- Ler `testing-integration.md` para critérios de adoção e escopo.

## Riscos Comuns
- Mock divergente do contrato real.
- Suite criada só por simetria, sem ganho real.
- Integração global em CI rápido para mudança localizada.
- Teste que valida detalhe interno em vez de comportamento observável.

## Proibido
- Teste dependente de ambiente compartilhado.
- Mocks escritos à mão quando o diff já exige mock gerado do contrato vigente.
- Rodar integração ampla por padrão sem relação direta com a superfície alterada.
