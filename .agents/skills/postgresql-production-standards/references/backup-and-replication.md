# Backup, Restore e Replicacao

## Fontes Oficiais
- Backup and restore: https://www.postgresql.org/docs/current/backup.html
- Logical backup: https://www.postgresql.org/docs/current/backup-dump.html
- File system level backup: https://www.postgresql.org/docs/current/backup-file.html
- Continuous archiving and PITR: https://www.postgresql.org/docs/current/continuous-archiving.html
- High availability, load balancing, replication: https://www.postgresql.org/docs/current/high-availability.html
- Warm standby and streaming replication: https://www.postgresql.org/docs/current/warm-standby.html
- Logical replication: https://www.postgresql.org/docs/current/logical-replication.html

## Regras Mandatorias
- Escolher estrategia de backup e restore a partir de RPO, RTO, tamanho do banco e objetivo operacional observavel.
- Tratar teste de restore como obrigatorio; backup sem restore comprovado nao e confiavel.
- Usar dump logico quando o objetivo for portabilidade logica e escopo administravel; usar backup fisico ou archiving quando o objetivo exigir recuperacao operacional mais forte.
- Escolher replicacao fisica ou logica a partir do objetivo observado: HA, leitura, distribuicao seletiva ou migracao.
- Monitorar lag, slots, WAL e estado de replicacao com mecanismos oficiais.

## Bloqueios Obrigatorios
- Bloquear recomendacao de HA ou replicacao sem definir objetivo: disponibilidade, leitura, migracao ou DR.
- Bloquear estrategia de backup sem RPO, RTO ou tamanho aproximado.
- Bloquear desenho de restore sem procedimento testavel e sem criterio de validacao pos-recuperacao.

## Evidencia Minima
- RPO e RTO alvo.
- Tamanho aproximado e crescimento.
- Objetivo operacional principal.
- Ambiente e restricoes de infraestrutura conhecidas.
