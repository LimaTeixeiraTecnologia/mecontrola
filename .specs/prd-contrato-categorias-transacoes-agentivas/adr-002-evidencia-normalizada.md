# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** Evidencia persistida em colunas normalizadas
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD exige evidencia funcional persistida junto da transacao ou template recorrente. O schema atual ja usa colunas explicitas e constraints para invariantes financeiras.

## Decisao

Persistir evidencia de categoria em colunas normalizadas nas tabelas `transactions` e `transactions_recurring_templates`, incluindo outcome, score, confidence, quality, signal type, source, reason, path e versao editorial. Nao usar JSONB como armazenamento primario. Complementar as colunas com FKs para `mecontrola.categories(id)` e triggers semanticos descritos na ADR-006.

## Alternativas Consideradas

- JSONB unico de evidencia: flexivel, mas reduz garantias por constraint e dificulta queries de auditoria.
- Log/evento apenas: util para observabilidade, mas nao cumpre persistencia junto do write.
- Tabela lateral de evidencias: normaliza ainda mais, mas aumenta joins e risco de write parcial.

## Consequencias

### Beneficios Esperados

- Auditoria direta por SQL.
- Constraints de enum e score no banco.
- Menor risco de write aprovado sem evidencia minima.
- Defesa contra bypass do use case por FK e trigger semantico.

### Trade-offs e Custos

- Mais colunas em duas tabelas.
- Repositorios e entidades precisam carregar mais campos.

### Riscos e Mitigacoes

- Risco: schema ficar verboso.
  Mitigacao: manter somente campos exigidos pelo PRD e usar nomes consistentes.

## Plano de Implementacao

1. Ajustar baseline de schema, pois o banco e novo.
2. Atualizar entidades e repositorios.
3. Adicionar FKs e triggers semanticos.
4. Adicionar testes de migracao e repositorio.

## Monitoramento e Validacao

Repository integration deve provar que todo write aceito persiste evidencia completa e que constraints rejeitam enum invalido.

## Impacto em Documentacao e Operacao

Documentar significado de cada coluna de evidencia para auditoria e suporte.

## Revisao Futura

Revisar se houver necessidade de historico multiplo de reclassificacoes por transacao.
