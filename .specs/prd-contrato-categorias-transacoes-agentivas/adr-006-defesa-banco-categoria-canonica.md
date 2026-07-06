# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** Defesa de banco para categoria canonica
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia
- **Relacionados:** `prd.md`, `techspec.md`, `adr-001-gate-unico-categoria.md`, `adr-005-subcategoria-folha-obrigatoria.md`

## Contexto

As migrations mostram que `categories` e `transactions` vivem no mesmo schema `mecontrola`. `categories` ja possui constraints e triggers para kind, parent e deprecated, mas `transactions` ainda nao possui FKs nem protecao semantica de banco para raiz, folha, kind, deprecated ou versao editorial.

## Decisao

Adicionar FKs de `transactions.category_id` e `transactions.subcategory_id` para `mecontrola.categories(id)` e aplicar a mesma protecao em `transactions_recurring_templates`. Adicionar triggers semanticos no baseline para validar raiz, folha direta, kind compativel com direction, deprecated_at nulo e versao editorial atual antes de `INSERT` e `UPDATE`.

## Alternativas Consideradas

- Confiar apenas no use case: simples, mas permite bypass por repository, migration manual ou bug futuro.
- Usar apenas FK: prova existencia, mas nao prova raiz/folha/kind/deprecated/version.
- Duplicar dados de categorias sem FK: facilita writes, mas quebra a autoridade canonica.

## Consequencias

### Beneficios Esperados

- Defesa em profundidade contra falso positivo.
- Banco rejeita writes invalidos mesmo se a aplicacao falhar.
- Auditoria e invariantes ficam verificaveis por SQL.

### Trade-offs e Custos

- Mais complexidade no baseline SQL.
- Testes de migracao e repositorio precisam cobrir triggers.

### Riscos e Mitigacoes

- Risco: triggers aumentarem custo de write.
  Mitigacao: leituras por PK em `categories` e tabela de versao unica; custo proporcional ao risco coberto.

## Plano de Implementacao

1. Alterar `000001_initial_schema.up.sql` para adicionar FKs.
2. Criar funcao SQL compartilhada para validar categoria de write.
3. Criar triggers em `transactions` e `transactions_recurring_templates`.
4. Atualizar testes de migration/integration para bypass direto de repository/use case.

## Monitoramento e Validacao

Integration tests devem provar que writes invalidos falham no banco mesmo quando executados diretamente contra repository/SQL.

## Impacto em Documentacao e Operacao

Documentar que qualquer carga operacional deve respeitar a mesma estrutura de raiz, folha, kind e versao editorial.

## Revisao Futura

Revisar se houver separacao fisica de bancos entre `categories` e `transactions`.
