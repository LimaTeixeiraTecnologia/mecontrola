# Manutencao e Observabilidade

## Fontes Oficiais
- Routine maintenance: https://www.postgresql.org/docs/current/maintenance.html
- Routine vacuuming: https://www.postgresql.org/docs/current/routine-vacuuming.html
- `VACUUM`: https://www.postgresql.org/docs/current/sql-vacuum.html
- `ANALYZE`: https://www.postgresql.org/docs/current/sql-analyze.html
- Statistics system: https://www.postgresql.org/docs/current/monitoring-stats.html
- Monitoring database activity: https://www.postgresql.org/docs/current/monitoring.html
- Resource consumption: https://www.postgresql.org/docs/current/runtime-config-resource.html

## Regras Mandatorias
- Considerar autovacuum e `ANALYZE` obrigatorios; desabilitar ou enfraquecer sem evidencia operacional e plano de compensacao e proibido.
- Tratar `VACUUM FULL` como excecao operacional; preferir rotina que mantenha estado estavel de espaco e estatisticas.
- Basear tuning em estatisticas, bloat, taxa de escrita, latencia e planos observados; nao em receitas genericas.
- Monitorar atividade, locks, vacuum, analyze e saude geral por visoes oficiais e roles apropriadas como `pg_monitor` quando aplicavel.
- Medir antes de alterar memoria, paralelismo ou custos; configuracao sem evidencia operacional viola a skill.

## Bloqueios Obrigatorios
- Bloquear tuning de `autovacuum`, memoria ou paralelismo sem sintoma observavel e sem metrica.
- Bloquear diagnostico de lentidao sem visoes, planos, logs ou estatisticas minimas.
- Bloquear recomendacao de manutencao invasiva sem janela, risco e estrategia de validacao.

## Evidencia Minima
- Sinais operacionais: views, logs, waits, planos ou contadores oficiais.
- Frequencia de escrita, leitura e crescimento.
- Ambiente afetado e urgencia operacional.
