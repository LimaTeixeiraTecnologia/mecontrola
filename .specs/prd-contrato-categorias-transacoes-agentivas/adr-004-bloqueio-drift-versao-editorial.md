# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** Bloqueio por drift de versao editorial
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O catalogo editorial de categorias possui versao. O PRD exige bloquear escrita quando a versao mudar entre classificacao/clarificacao e persistencia.

## Decisao

Toda evidencia de categoria carrega `category_editorial_version`. Antes de persistir, o gate compara essa versao com a versao atual de `categories`. Divergencia gera bloqueio tipado `version_changed`, recarrega candidatos e exige nova confirmacao ou clarificacao.

## Alternativas Consideradas

- Aceitar a categoria se os IDs ainda existirem: reduz bloqueios, mas ignora mudanca semantica editorial.
- Revalidar silenciosamente e persistir: conveniente, mas pode trocar o fundamento da decisao sem consentimento.
- Usar TTL em vez de versao: simples, mas nao detecta mudanca editorial real com precisao.

## Consequencias

### Beneficios Esperados

- Decisao auditavel contra uma versao especifica do catalogo.
- Bloqueio deterministico em corrida editorial.
- Reducao de falso positivo por catalogo alterado.

### Trade-offs e Custos

- Pode aumentar clarificacoes durante curadoria de categorias.
- Exige leitura de versao proxima da persistencia.

### Riscos e Mitigacoes

- Risco: bloqueio excessivo em edicoes frequentes do catalogo.
  Mitigacao: metricar `category_write_version_drift_total` e revisar processo editorial.

## Plano de Implementacao

1. Propagar version no output de classificacao.
2. Incluir expected version no gate de escrita.
3. Persistir version na evidencia.
4. Testar corrida entre classificacao e escrita.

## Monitoramento e Validacao

Monitorar `category_write_version_drift_total{kind,surface}` e tratar qualquer ocorrencia como bloqueio funcional esperado que exige nova resolucao antes de persistencia.

## Impacto em Documentacao e Operacao

Runbook de suporte deve orientar recarregar candidatos e pedir nova confirmacao.

## Revisao Futura

Revisar se a curadoria editorial passar a ocorrer durante alto volume de escritas e a metrica de drift indicar impacto operacional recorrente.
