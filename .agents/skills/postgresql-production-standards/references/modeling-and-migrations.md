# Modelagem e Migrations

## Fontes Oficiais
- Tipos de dados: https://www.postgresql.org/docs/current/datatype.html
- Constraints: https://www.postgresql.org/docs/current/ddl-constraints.html
- Defaults: https://www.postgresql.org/docs/current/ddl-default.html
- Generated columns: https://www.postgresql.org/docs/current/ddl-generated-columns.html
- ALTER TABLE: https://www.postgresql.org/docs/current/sql-altertable.html

## Regras Mandatorias
- Definir `PRIMARY KEY` em toda tabela persistente, salvo evidencia objetiva de tabela temporaria ou staging.
- Usar tipos nativos do PostgreSQL coerentes com o dominio real; rejeitar `text` universal para mascarar falta de modelagem.
- Aplicar `NOT NULL` sempre que a ausencia de valor nao fizer parte do contrato de negocio.
- Aplicar `UNIQUE`, `CHECK` e `FOREIGN KEY` no banco quando a integridade for obrigatoria; nao deslocar integridade essencial apenas para a aplicacao.
- Usar colunas derivadas ou generated columns apenas quando a regra oficial e a necessidade observada justificarem o custo de escrita e manutencao.
- Exigir rollout e reversao planejados quando a migration alterar contratos, volumes relevantes ou janelas de lock sensiveis.
- Separar mudancas destrutivas em etapas quando houver risco de lock prolongado, backfill ou incompatibilidade temporaria entre app e schema.

## Bloqueios Obrigatorios
- Bloquear quando o pedido de schema nao informar cardinalidade, unicidade, nulabilidade, relacionamentos ou estrategia de rollout.
- Bloquear quando a migration tocar tabela grande sem estrategia observavel para lock, backfill e validacao.
- Bloquear quando o pedido exigir extensao nao coberta explicitamente pela documentacao oficial usada no projeto.

## Evidencia Minima
- DDL atual ou migration proposta.
- Contrato funcional minimo: unicidade, nulabilidade, relacionamentos e semantica de exclusao.
- Quando houver alteracao em producao: volume aproximado e janela de rollout.
